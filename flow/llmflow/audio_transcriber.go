// Copyright 2025 The Go A2A Authors
// SPDX-License-Identifier: Apache-2.0

package llmflow

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"cloud.google.com/go/auth/credentials"
	speech "cloud.google.com/go/speech/apiv1"
	speechpb "cloud.google.com/go/speech/apiv1/speechpb"
	"google.golang.org/api/option"
	"google.golang.org/genai"

	"github.com/xenoweaver/adk-go/model"
	"github.com/xenoweaver/adk-go/types"
)

// AudioTranscriber represents a transcribes audio using Google Cloud Speech-to-Text.
type AudioTranscriber struct {
	client *speech.Client
}

// NewAudioTranscriber creates a new [AudioTranscriber] instance.
func NewAudioTranscriber(ctx context.Context) (*AudioTranscriber, error) {
	creds, err := credentials.DetectDefault(&credentials.DetectOptions{
		Scopes: speech.DefaultAuthScopes(),
	})
	if err != nil {
		return nil, fmt.Errorf("get credentials for speech: %w", err)
	}

	client, err := speech.NewClient(ctx, option.WithAuthCredentials(creds))
	if err != nil {
		return nil, fmt.Errorf("create gRPC speech client: %w", err)
	}

	return &AudioTranscriber{
		client: client,
	}, nil
}

// TranscribeFile transcribe audio, bundling consecutive segments from the same speaker.
//
// The ordering of speakers will be preserved. Audio blobs will be merged for
// the same speaker as much as we can do reduce the transcription latency.
func (f *AudioTranscriber) TranscribeFile(ctx context.Context, ictx *types.InvocationContext) ([]*genai.Content, error) {
	bundledAudio := make(map[string]any)
	currentSpeaker := ""
	currentAudioData := new(bytes.Buffer)
	contents := []*genai.Content{}

	// Step1: merge audio blobs
	for _, transactionEntry := range ictx.TranscriptionCache {
		speaker := transactionEntry.Role
		audioData := transactionEntry.Data

		switch audioData := audioData.(type) {
		case *genai.Content:
			if currentSpeaker != "" {
				bundledAudio[currentSpeaker] = currentAudioData
			}
			bundledAudio[speaker] = audioData
			continue

		case *genai.Blob:
			if audioData.Data == nil {
				continue
			}

			switch {
			case speaker == currentSpeaker:
				currentAudioData.Write(audioData.Data)
			default:
				if currentSpeaker != "" {
					bundledAudio[currentSpeaker] = currentAudioData
				}
				currentSpeaker = speaker
				currentAudioData.Reset()
				currentAudioData.Write(audioData.Data)
			}
		}
	}

	// Append the last audio segment if any
	if currentSpeaker != "" {
		bundledAudio[currentSpeaker] = currentAudioData
	}

	// reset cache
	clear(ictx.TranscriptionCache)

	// Step2: transcription
	for speaker, data := range bundledAudio {
		switch data := data.(type) {
		case *genai.Blob:
			audio := &speechpb.RecognitionAudio{
				AudioSource: &speechpb.RecognitionAudio_Content{
					Content: data.Data,
				},
			}
			config := &speechpb.RecognitionConfig{
				Encoding:        speechpb.RecognitionConfig_LINEAR16,
				SampleRateHertz: 16000,
				LanguageCode:    "en-US",
			}
			req := &speechpb.RecognizeRequest{
				Config: config,
				Audio:  audio,
			}

			response, err := f.client.Recognize(ctx, req)
			if err != nil {
				return nil, err
			}

			for _, result := range response.Results {
				transcript := result.Alternatives[0].Transcript
				parts := []*genai.Part{genai.NewPartFromText(transcript)}
				role := strings.ToLower(speaker)
				content := genai.NewContentFromParts(parts, model.ToGenAIRole(role))
				contents = append(contents, content)
			}
		case *genai.Content:
			contents = append(contents, data)
		}
	}

	return contents, nil
}
