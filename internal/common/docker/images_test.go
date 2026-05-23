// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package docker

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestParseImageName(t *testing.T) {
	tests := []struct {
		name         string
		image        string
		wantImageRef ImageRef
		wantErr      bool
	}{
		{
			name:  "empty string",
			image: "",
			wantImageRef: ImageRef{
				Repository:      "",
				Tag:             "",
				Digest:          "",
				DigestAlgorithm: "",
			},
			wantErr: true,
		},
		{
			name:  "malformed image",
			image: "aaa:",
			wantImageRef: ImageRef{
				Repository:      "",
				Tag:             "",
				Digest:          "",
				DigestAlgorithm: "",
			},
			wantErr: true,
		},
		{
			name:  "shorthand only",
			image: "alpine",
			wantImageRef: ImageRef{
				Repository:      "alpine",
				Tag:             "latest",
				Digest:          "",
				DigestAlgorithm: "",
			},
			wantErr: false,
		},
		{
			name:  "shorthand with tag",
			image: "alpine:v1.0.0",
			wantImageRef: ImageRef{
				Repository:      "alpine",
				Tag:             "v1.0.0",
				Digest:          "",
				DigestAlgorithm: "",
			},
			wantErr: false,
		},
		{
			name:  "repository without registry and tag",
			image: "alpine/alpine",
			wantImageRef: ImageRef{
				Repository:      "alpine/alpine",
				Tag:             "latest",
				Digest:          "",
				DigestAlgorithm: "",
			},
			wantErr: false,
		},
		{
			name:  "repository without registry",
			image: "alpine/alpine:2.0.0",
			wantImageRef: ImageRef{
				Repository:      "alpine/alpine",
				Tag:             "2.0.0",
				Digest:          "",
				DigestAlgorithm: "",
			},
			wantErr: false,
		},
		{
			name:  "repository with registry and without tag",
			image: "example.com/alpine/alpine",
			wantImageRef: ImageRef{
				Repository:      "example.com/alpine/alpine",
				Tag:             "latest",
				Digest:          "",
				DigestAlgorithm: "",
			},
			wantErr: false,
		},
		{
			name:  "repository with registry and tag",
			image: "example.com/alpine/alpine:1",
			wantImageRef: ImageRef{
				Repository:      "example.com/alpine/alpine",
				Tag:             "1",
				Digest:          "",
				DigestAlgorithm: "",
			},
			wantErr: false,
		},
		{
			name:  "repository with registry and port but without tag",
			image: "example.com:3000/alpine/alpine",
			wantImageRef: ImageRef{
				Repository:      "example.com:3000/alpine/alpine",
				Tag:             "latest",
				Digest:          "",
				DigestAlgorithm: "",
			},
			wantErr: false,
		},
		{
			name:  "repository with registry, port and tag",
			image: "example.com:3000/alpine/alpine:test",
			wantImageRef: ImageRef{
				Repository:      "example.com:3000/alpine/alpine",
				Tag:             "test",
				Digest:          "",
				DigestAlgorithm: "",
			},
			wantErr: false,
		},
		{
			name:  "image with sha256 hash",
			image: "alpine:test@sha256:dbc66f8c46d4cf4793527ca0737d73527a2bb830019953c2371b5f45f515f1a8",
			wantImageRef: ImageRef{
				Repository:      "alpine",
				Tag:             "test",
				Digest:          "dbc66f8c46d4cf4793527ca0737d73527a2bb830019953c2371b5f45f515f1a8",
				DigestAlgorithm: "sha256",
			},
			wantErr: false,
		},
		{
			name:  "image with repository and hash",
			image: "example.com/alpine/alpine@sha256:dbc66f8c46d4cf4793527ca0737d73527a2bb830019953c2371b5f45f515f1a8",
			wantImageRef: ImageRef{
				Repository:      "example.com/alpine/alpine",
				Tag:             "latest",
				Digest:          "dbc66f8c46d4cf4793527ca0737d73527a2bb830019953c2371b5f45f515f1a8",
				DigestAlgorithm: "sha256",
			},
			wantErr: false,
		},
		{
			name:  "image with repository, tag and hash",
			image: "example.com/alpine/alpine:1.0.0@sha256:dbc66f8c46d4cf4793527ca0737d73527a2bb830019953c2371b5f45f515f1a8",
			wantImageRef: ImageRef{
				Repository:      "example.com/alpine/alpine",
				Tag:             "1.0.0",
				Digest:          "dbc66f8c46d4cf4793527ca0737d73527a2bb830019953c2371b5f45f515f1a8",
				DigestAlgorithm: "sha256",
			},
			wantErr: false,
		},
		{
			name:  "image with repository, tag and SHA512 hash",
			image: "example.com/alpine/alpine:1.0.0@sha512:3d425c5a102d441da33030949ba5ec22e388ed0529c298a1984d62486d4924806949708b834229206ee5a36ba30f6de6d09989019e5790a8b665539f9489efd5",
			wantImageRef: ImageRef{
				Repository:      "example.com/alpine/alpine",
				Tag:             "1.0.0",
				Digest:          "3d425c5a102d441da33030949ba5ec22e388ed0529c298a1984d62486d4924806949708b834229206ee5a36ba30f6de6d09989019e5790a8b665539f9489efd5",
				DigestAlgorithm: "sha512",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			image, err := ParseImageName(tt.image)
			if !tt.wantErr {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantImageRef, image)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestCanonicalImageRef(t *testing.T) {
	tests := []struct {
		name      string
		image     string
		wantImage string
		wantErr   bool
	}{
		{
			name:      "empty string",
			image:     "",
			wantImage: "",
			wantErr:   true,
		},
		{
			name:      "malformed image",
			image:     "aaa:",
			wantImage: "",
			wantErr:   true,
		},
		{
			name:      "shorthand only",
			image:     "alpine",
			wantImage: "",
			wantErr:   true,
		},
		{
			name:      "shorthand with tag",
			image:     "alpine:v1.0.0",
			wantImage: "",
			wantErr:   true,
		},
		{
			name:      "repository without registry and tag",
			image:     "alpine/alpine",
			wantImage: "",
			wantErr:   true,
		},
		{
			name:      "repository without registry",
			image:     "alpine/alpine:2.0.0",
			wantImage: "",
			wantErr:   true,
		},
		{
			name:      "repository with registry and without tag",
			image:     "example.com/alpine/alpine",
			wantImage: "",
			wantErr:   true,
		},
		{
			name:      "repository with registry and tag",
			image:     "example.com/alpine/alpine:1",
			wantImage: "",
			wantErr:   true,
		},
		{
			name:      "repository with registry and port but without tag",
			image:     "example.com:3000/alpine/alpine",
			wantImage: "",
			wantErr:   true,
		},
		{
			name:      "repository with registry, port, tag and hash",
			image:     "example.com:3000/alpine/alpine:test@sha256:dbc66f8c46d4cf4793527ca0737d73527a2bb830019953c2371b5f45f515f1a8",
			wantImage: "example.com:3000/alpine/alpine:test@sha256:dbc66f8c46d4cf4793527ca0737d73527a2bb830019953c2371b5f45f515f1a8",
			wantErr:   false,
		},
		{
			name:      "image with hash",
			image:     "alpine@sha256:dbc66f8c46d4cf4793527ca0737d73527a2bb830019953c2371b5f45f515f1a8",
			wantImage: "docker.io/library/alpine@sha256:dbc66f8c46d4cf4793527ca0737d73527a2bb830019953c2371b5f45f515f1a8",
			wantErr:   false,
		},
		{
			name:      "image with tag and hash",
			image:     "alpine:test@sha256:dbc66f8c46d4cf4793527ca0737d73527a2bb830019953c2371b5f45f515f1a8",
			wantImage: "docker.io/library/alpine:test@sha256:dbc66f8c46d4cf4793527ca0737d73527a2bb830019953c2371b5f45f515f1a8",
			wantErr:   false,
		},
		{
			name:      "image with repository and hash",
			image:     "example.com/alpine/alpine@sha256:dbc66f8c46d4cf4793527ca0737d73527a2bb830019953c2371b5f45f515f1a8",
			wantImage: "example.com/alpine/alpine@sha256:dbc66f8c46d4cf4793527ca0737d73527a2bb830019953c2371b5f45f515f1a8",
			wantErr:   false,
		},
		{
			name:      "image with repository, tag and hash",
			image:     "example.com/alpine/alpine:1.0.0@sha256:dbc66f8c46d4cf4793527ca0737d73527a2bb830019953c2371b5f45f515f1a8",
			wantImage: "example.com/alpine/alpine:1.0.0@sha256:dbc66f8c46d4cf4793527ca0737d73527a2bb830019953c2371b5f45f515f1a8",
			wantErr:   false,
		},
		{
			name:      "image with repository, tag and SHA512 hash",
			image:     "example.com/alpine/alpine:1.0.0@sha512:3d425c5a102d441da33030949ba5ec22e388ed0529c298a1984d62486d4924806949708b834229206ee5a36ba30f6de6d09989019e5790a8b665539f9489efd5",
			wantImage: "example.com/alpine/alpine:1.0.0@sha512:3d425c5a102d441da33030949ba5ec22e388ed0529c298a1984d62486d4924806949708b834229206ee5a36ba30f6de6d09989019e5790a8b665539f9489efd5",
			wantErr:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			image, err := CanonicalImageRef(tt.image)
			if !tt.wantErr {
				require.NoError(t, err)
				require.Equal(t, tt.wantImage, image)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestLogParseError(t *testing.T) {
	// Create an observer and a logger with it
	core, observedLogs := observer.New(zap.DebugLevel)
	logger := zap.New(core)

	// Inputs
	err := errors.New("test parse error")
	image := "alpine:latest"

	// Call the function
	LogParseError(err, image, logger)

	// Assertions
	logs := observedLogs.All()
	assert.Len(t, logs, 1, "expected 1 log entry, got %d", len(logs))

	entry := logs[0]
	assert.ErrorContains(t, err, entry.Message, "expected log message %q, got %q", err.Error(), entry.Message)

	fields := entry.ContextMap()
	img, ok := fields["image"]
	assert.True(t, ok, "expected field 'image' to be present")
	assert.Equal(t, image, img, "expected field 'image' to have value %q, got %v", image, img)
}
