// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDockerImageToElements(t *testing.T) {
	type args struct {
		image string
	}
	tests := []struct {
		name           string
		args           args
		wantRepository string
		wantTag        string
		wantSHA256     string
		wantErr        bool
	}{
		{
			name: "empty string",
			args: args{
				image: "",
			},
			wantRepository: "",
			wantTag:        "",
			wantSHA256:     "",
			wantErr:        true,
		},
		{
			name: "malformed image",
			args: args{
				image: "aaa:",
			},
			wantRepository: "",
			wantTag:        "",
			wantSHA256:     "",
			wantErr:        true,
		},
		{
			name: "image with sha256 hash",
			args: args{
				image: "alpine:test@sha256:dbc66f8c46d4cf4793527ca0737d73527a2bb830019953c2371b5f45f515f1a8",
			},
			wantRepository: "alpine",
			wantTag:        "test",
			wantSHA256:     "dbc66f8c46d4cf4793527ca0737d73527a2bb830019953c2371b5f45f515f1a8",
			wantErr:        false,
		},
		{
			name: "shorthand only",
			args: args{
				image: "alpine",
			},
			wantRepository: "alpine",
			wantTag:        "latest",
			wantSHA256:     "",
			wantErr:        false,
		},
		{
			name: "shorthand with tag",
			args: args{
				image: "alpine:v1.0.0",
			},
			wantRepository: "alpine",
			wantTag:        "v1.0.0",
			wantSHA256:     "",
			wantErr:        false,
		},
		{
			name: "repository without registry and tag",
			args: args{
				image: "alpine/alpine",
			},
			wantRepository: "alpine/alpine",
			wantTag:        "latest",
			wantSHA256:     "",
			wantErr:        false,
		},
		{
			name: "repository without registry",
			args: args{
				image: "alpine/alpine:2.0.0",
			},
			wantRepository: "alpine/alpine",
			wantTag:        "2.0.0",
			wantSHA256:     "",
			wantErr:        false,
		},
		{
			name: "repository with registry and without tag",
			args: args{
				image: "example.com/alpine/alpine",
			},
			wantRepository: "example.com/alpine/alpine",
			wantTag:        "latest",
			wantSHA256:     "",
			wantErr:        false,
		},
		{
			name: "repository with registry and tag",
			args: args{
				image: "example.com/alpine/alpine:1",
			},
			wantRepository: "example.com/alpine/alpine",
			wantTag:        "1",
			wantSHA256:     "",
			wantErr:        false,
		},
		{
			name: "repository with registry and port but without tag",
			args: args{
				image: "example.com:3000/alpine/alpine",
			},
			wantRepository: "example.com:3000/alpine/alpine",
			wantTag:        "latest",
			wantSHA256:     "",
			wantErr:        false,
		},
		{
			name: "repository with registry, port and tag",
			args: args{
				image: "example.com:3000/alpine/alpine:test",
			},
			wantRepository: "example.com:3000/alpine/alpine",
			wantTag:        "test",
			wantSHA256:     "",
			wantErr:        false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			image, err := ParseImageName(tt.args.image)
			if !tt.wantErr {
				assert.NoError(t, err, "ParseImageName() error = %v, wantErr %v", err, tt.wantErr)
				assert.Equal(t, tt.wantRepository, image.Repository, "ParseImageName() repository = %v, want %v", image.Repository, tt.wantRepository)
				assert.Equal(t, tt.wantTag, image.Tag, "ParseImageName() tag = %v, want %v", image.Tag, tt.wantTag)
				assert.Equal(t, tt.wantSHA256, image.SHA256, "ParseImageName() hash = %v, want %v", image.SHA256, tt.wantSHA256)
			} else {
				require.Error(t, err)
			}
		})
	}
}
