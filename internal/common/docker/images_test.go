// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package docker

import (
	"testing"
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
				image: "alpine:test@sha256:00000000000000",
			},
			wantRepository: "alpine",
			wantTag:        "test",
			wantSHA256:     "00000000000000",
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
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseImageName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if image.Repository != tt.wantRepository {
				t.Errorf("ParseImageName() repository = %v, want %v", image.Repository, tt.wantRepository)
			}
			if image.Tag != tt.wantTag {
				t.Errorf("ParseImageName() tag = %v, want %v", image.Tag, tt.wantTag)
			}
			if image.SHA256 != tt.wantSHA256 {
				t.Errorf("ParseImageName() hash = %v, want %v", image.SHA256, tt.wantSHA256)
			}
		})
	}
}
