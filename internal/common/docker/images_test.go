// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseImageName(t *testing.T) {
	tests := []struct {
		name           string
		image          string
		wantRepository string
		wantTag        string
		wantDigest     string
		wantAlgorithm  string
		wantErr        bool
	}{
		{
			name:           "empty string",
			image:          "",
			wantRepository: "",
			wantTag:        "",
			wantDigest:     "",
			wantAlgorithm:  "",
			wantErr:        true,
		},
		{
			name:           "malformed image",
			image:          "aaa:",
			wantRepository: "",
			wantTag:        "",
			wantDigest:     "",
			wantAlgorithm:  "",
			wantErr:        true,
		},
		{
			name:           "shorthand only",
			image:          "alpine",
			wantRepository: "alpine",
			wantTag:        "latest",
			wantDigest:     "",
			wantAlgorithm:  "",
			wantErr:        false,
		},
		{
			name:           "shorthand with tag",
			image:          "alpine:v1.0.0",
			wantRepository: "alpine",
			wantTag:        "v1.0.0",
			wantDigest:     "",
			wantAlgorithm:  "",
			wantErr:        false,
		},
		{
			name:           "repository without registry and tag",
			image:          "alpine/alpine",
			wantRepository: "alpine/alpine",
			wantTag:        "latest",
			wantDigest:     "",
			wantAlgorithm:  "",
			wantErr:        false,
		},
		{
			name:           "repository without registry",
			image:          "alpine/alpine:2.0.0",
			wantRepository: "alpine/alpine",
			wantTag:        "2.0.0",
			wantDigest:     "",
			wantAlgorithm:  "",
			wantErr:        false,
		},
		{
			name:           "repository with registry and without tag",
			image:          "example.com/alpine/alpine",
			wantRepository: "example.com/alpine/alpine",
			wantTag:        "latest",
			wantDigest:     "",
			wantAlgorithm:  "",
			wantErr:        false,
		},
		{
			name:           "repository with registry and tag",
			image:          "example.com/alpine/alpine:1",
			wantRepository: "example.com/alpine/alpine",
			wantTag:        "1",
			wantDigest:     "",
			wantAlgorithm:  "",
			wantErr:        false,
		},
		{
			name:           "repository with registry and port but without tag",
			image:          "example.com:3000/alpine/alpine",
			wantRepository: "example.com:3000/alpine/alpine",
			wantTag:        "latest",
			wantDigest:     "",
			wantAlgorithm:  "",
			wantErr:        false,
		},
		{
			name:           "repository with registry, port and tag",
			image:          "example.com:3000/alpine/alpine:test",
			wantRepository: "example.com:3000/alpine/alpine",
			wantTag:        "test",
			wantDigest:     "",
			wantAlgorithm:  "",
			wantErr:        false,
		},
		{
			name:           "image with sha256 hash",
			image:          "alpine:test@sha256:dbc66f8c46d4cf4793527ca0737d73527a2bb830019953c2371b5f45f515f1a8",
			wantRepository: "alpine",
			wantTag:        "test",
			wantDigest:     "dbc66f8c46d4cf4793527ca0737d73527a2bb830019953c2371b5f45f515f1a8",
			wantAlgorithm:  "sha256",
			wantErr:        false,
		},
		{
			name:           "image with repository and hash",
			image:          "example.com/alpine/alpine@sha256:dbc66f8c46d4cf4793527ca0737d73527a2bb830019953c2371b5f45f515f1a8",
			wantRepository: "example.com/alpine/alpine",
			wantTag:        "latest",
			wantDigest:     "dbc66f8c46d4cf4793527ca0737d73527a2bb830019953c2371b5f45f515f1a8",
			wantAlgorithm:  "sha256",
			wantErr:        false,
		},
		{
			name:           "image with repository, tag and hash",
			image:          "example.com/alpine/alpine:1.0.0@sha256:dbc66f8c46d4cf4793527ca0737d73527a2bb830019953c2371b5f45f515f1a8",
			wantRepository: "example.com/alpine/alpine",
			wantTag:        "1.0.0",
			wantDigest:     "dbc66f8c46d4cf4793527ca0737d73527a2bb830019953c2371b5f45f515f1a8",
			wantAlgorithm:  "sha256",
			wantErr:        false,
		},
		{
			name:           "image with repository, tag and SHA512 hash",
			image:          "example.com/alpine/alpine:1.0.0@sha512:3d425c5a102d441da33030949ba5ec22e388ed0529c298a1984d62486d4924806949708b834229206ee5a36ba30f6de6d09989019e5790a8b665539f9489efd5",
			wantRepository: "example.com/alpine/alpine",
			wantTag:        "1.0.0",
			wantDigest:     "3d425c5a102d441da33030949ba5ec22e388ed0529c298a1984d62486d4924806949708b834229206ee5a36ba30f6de6d09989019e5790a8b665539f9489efd5",
			wantAlgorithm:  "sha512",
			wantErr:        false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			image, err := ParseImageName(tt.image)
			if !tt.wantErr {
				assert.NoError(t, err, "ParseImageName() error = %v, wantErr %v", err, tt.wantErr)
				assert.Equal(t, tt.wantRepository, image.Repository, "ParseImageName() repository = %v, want %v", image.Repository, tt.wantRepository)
				assert.Equal(t, tt.wantTag, image.Tag, "ParseImageName() tag = %v, want %v", image.Tag, tt.wantTag)
				assert.Equal(t, tt.wantDigest, image.Digest, "ParseImageName() hash = %v, want %v", image.Digest, tt.wantDigest)
				assert.Equal(t, tt.wantAlgorithm, image.DigestAlgorithm, "ParseImageName() hash algorithm = %v, want %v", image.DigestAlgorithm, tt.wantAlgorithm)
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
			wantImage: "alpine",
			wantErr:   false,
		},
		{
			name:      "shorthand with tag",
			image:     "alpine:v1.0.0",
			wantImage: "alpine",
			wantErr:   false,
		},
		{
			name:      "repository without registry and tag",
			image:     "alpine/alpine",
			wantImage: "alpine/alpine",
			wantErr:   false,
		},
		{
			name:      "repository without registry",
			image:     "alpine/alpine:2.0.0",
			wantImage: "alpine/alpine",
			wantErr:   false,
		},
		{
			name:      "repository with registry and without tag",
			image:     "example.com/alpine/alpine",
			wantImage: "example.com/alpine/alpine",
			wantErr:   false,
		},
		{
			name:      "repository with registry and tag",
			image:     "example.com/alpine/alpine:1",
			wantImage: "example.com/alpine/alpine",
			wantErr:   false,
		},
		{
			name:      "repository with registry and port but without tag",
			image:     "example.com:3000/alpine/alpine",
			wantImage: "example.com:3000/alpine/alpine",
			wantErr:   false,
		},
		{
			name:      "repository with registry, port and tag",
			image:     "example.com:3000/alpine/alpine:test",
			wantImage: "example.com:3000/alpine/alpine",
			wantErr:   false,
		},
		{
			name:      "image with sha256 hash",
			image:     "alpine:test@sha256:dbc66f8c46d4cf4793527ca0737d73527a2bb830019953c2371b5f45f515f1a8",
			wantImage: "alpine",
			wantErr:   false,
		},
		{
			name:      "image with repository and hash",
			image:     "example.com/alpine/alpine@sha256:dbc66f8c46d4cf4793527ca0737d73527a2bb830019953c2371b5f45f515f1a8",
			wantImage: "example.com/alpine/alpine",
			wantErr:   false,
		},
		{
			name:      "image with repository, tag and hash",
			image:     "example.com/alpine/alpine:1.0.0@sha256:dbc66f8c46d4cf4793527ca0737d73527a2bb830019953c2371b5f45f515f1a8",
			wantImage: "example.com/alpine/alpine",
			wantErr:   false,
		},
		{
			name:      "image with repository, tag and SHA512 hash",
			image:     "example.com/alpine/alpine:1.0.0@sha512:3d425c5a102d441da33030949ba5ec22e388ed0529c298a1984d62486d4924806949708b834229206ee5a36ba30f6de6d09989019e5790a8b665539f9489efd5",
			wantImage: "example.com/alpine/alpine",
			wantErr:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			image, err := ParseImageName(tt.image)
			if !tt.wantErr {
				assert.NoError(t, err, "ParseImageName() error = %v, wantErr %v", err, tt.wantErr)
				assert.Equal(t, tt.wantImage, image.Repository, "ParseImageName() image reference = %v, want %v", image.Repository, tt.wantImage)
			} else {
				require.Error(t, err)
			}
		})
	}
}
