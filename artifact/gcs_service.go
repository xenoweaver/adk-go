// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package artifact

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"cloud.google.com/go/auth/credentials"
	"cloud.google.com/go/storage"
	"golang.org/x/sync/errgroup"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/genai"

	"github.com/xenoweaver/adk-go/pkg/py"
	"github.com/xenoweaver/adk-go/types"
)

// GCSService represents an artifact service implementation using Google Cloud Storage (GCS).
type GCSService struct {
	client *storage.Client
	bucket *storage.BucketHandle
}

var _ types.ArtifactService = (*GCSService)(nil)

// NewGCSService creates a new [GCSService] instance with the given bucket name.
func NewGCSService(ctx context.Context, bucketName string) (*GCSService, error) {
	creds, err := credentials.DetectDefault(&credentials.DetectOptions{
		Scopes: []string{
			storage.ScopeFullControl,
			storage.ScopeReadWrite,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("get credentials for storage: %w", err)
	}

	client, err := storage.NewGRPCClient(ctx, option.WithAuthCredentials(creds))
	if err != nil {
		return nil, fmt.Errorf("create storage client: %w", err)
	}
	bucket := client.Bucket(bucketName)

	return &GCSService{
		client: client,
		bucket: bucket,
	}, nil
}

// fileHasUserNamespace checks if the filename has a user namespace.
func (a *GCSService) fileHasUserNamespace(filename string) bool {
	return strings.HasPrefix(filename, "user:")
}

// getBlobName constructs the blob name in GCS.
func (a *GCSService) getBlobName(appName, userID, sessionID, filename string, version int) string {
	if a.fileHasUserNamespace(filename) {
		return fmt.Sprintf("%s/%s/user/%s/%d", appName, userID, filename, version)
	}
	return fmt.Sprintf("%s/%s/%s/%s/%d", appName, userID, sessionID, filename, version)
}

// SaveArtifact implements [types.ArtifactService].
func (a *GCSService) SaveArtifact(ctx context.Context, appName, userID, sessionID, filename string, artifact *genai.Part) (int, error) {
	versions, err := a.ListVersions(ctx, appName, userID, sessionID, filename)
	if err != nil {
		return 0, err
	}
	version := 0
	if len(versions) > 0 {
		version = len(versions) - 1
	}

	blobName := a.getBlobName(appName, userID, sessionID, filename, version)
	blob := a.bucket.Object(blobName)

	w := blob.NewWriter(ctx)
	defer w.Close()
	if _, err := io.Copy(w, bytes.NewReader(artifact.InlineData.Data)); err != nil {
		return 0, err
	}

	if _, err := blob.Update(ctx, storage.ObjectAttrsToUpdate{
		ContentType: artifact.InlineData.MIMEType,
	}); err != nil {
		return 0, err
	}

	return version, nil
}

// LoadArtifact implements [types.ArtifactService].
func (a *GCSService) LoadArtifact(ctx context.Context, appName, userID, sessionID, filename string, version int) (*genai.Part, error) {
	if version == 0 {
		versions, err := a.ListVersions(ctx, appName, userID, sessionID, filename)
		if err != nil {
			return nil, err
		}
		slices.Reverse(versions)
		version = versions[len(versions)-1]
	}

	blobName := a.getBlobName(appName, userID, sessionID, filename, version)
	blob := a.bucket.Object(blobName)

	r, err := blob.NewReader(ctx)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	attrs, err := blob.Attrs(ctx)
	if err != nil {
		return nil, err
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	artifact := genai.NewPartFromBytes(data, attrs.ContentType)

	return artifact, nil
}

// ListArtifactKey implements [types.ArtifactService].
func (a *GCSService) ListArtifactKey(ctx context.Context, appName, userID, sessionID string) ([]string, error) {
	filenames := py.NewSet[string]()

	eg, ctx := errgroup.WithContext(ctx)

	sessionFilename := py.NewSet[string]()
	eg.Go(func() error {
		sessionPrefix := fmt.Sprintf("%s/%s/%s/", appName, userID, sessionID)
		sessionBlobsIt := a.bucket.Objects(ctx, &storage.Query{
			Prefix: sessionPrefix,
		})
		for {
			objAttrs, err := sessionBlobsIt.Next()
			if err != nil {
				if errors.Is(err, iterator.Done) {
					break
				}
				return err
			}

			filename := objAttrs.Name
			if pairs := strings.Split(filename, "/"); len(pairs) == 5 {
				sessionFilename.Insert(pairs[3])
			}
		}
		return nil
	})

	userNamespaceFilename := py.NewSet[string]()
	eg.Go(func() error {
		userNamespacePrefix := fmt.Sprintf("%s/%s/user/", appName, userID)
		userNamespaceBlobs := a.bucket.Objects(ctx, &storage.Query{
			Prefix: userNamespacePrefix,
		})
		for {
			objAttrs, err := userNamespaceBlobs.Next()
			if err != nil {
				if errors.Is(err, iterator.Done) {
					break
				}
				return err
			}

			filename := objAttrs.Name
			if pairs := strings.Split(filename, "/"); len(pairs) == 5 {
				userNamespaceFilename.Insert(pairs[3])
			}
		}
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	filenames.Insert(sessionFilename.UnsortedList()...)
	filenames.Insert(userNamespaceFilename.UnsortedList()...)

	return py.List(filenames), nil
}

// DeleteArtifact implements [types.ArtifactService].
func (a *GCSService) DeleteArtifact(ctx context.Context, appName, userID, sessionID, filename string) error {
	versions, err := a.ListVersions(ctx, appName, userID, sessionID, filename)
	if err != nil {
		return err
	}

	for _, version := range versions {
		blobName := a.getBlobName(appName, userID, sessionID, filename, version)
		blob := a.bucket.Object(blobName)
		if err := blob.Delete(ctx); err != nil {
			return err
		}
	}

	return nil
}

// ListVersions implements [types.ArtifactService].
func (a *GCSService) ListVersions(ctx context.Context, appName, userID, sessionID, filename string) ([]int, error) {
	prefix := a.getBlobName(appName, userID, sessionID, filename, 0)
	it := a.bucket.Objects(ctx, &storage.Query{
		Prefix: prefix,
	})

	blobNames := []string{}
	for {
		objAttrs, err := it.Next()
		if err != nil {
			if errors.Is(err, iterator.Done) {
				break
			}
			return nil, err
		}

		blobNames = append(blobNames, objAttrs.Name)
	}

	versions := make([]int, len(blobNames))
	for i, blobName := range blobNames {
		idx := strings.LastIndex(blobName, "/")
		version, err := strconv.Atoi(blobName[idx+1:])
		if err != nil {
			return nil, err
		}
		versions[i] = version
	}

	return versions, nil
}

// Close implements [types.ArtifactService].
func (a *GCSService) Close() error {
	return a.client.Close()
}
