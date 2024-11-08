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
		wantDigest     string
		wantErr        bool
	}{
		{
			name: "empty string",
			args: args{
				image: "",
			},
			wantRepository: "",
			wantTag:        "",
			wantDigest:     "",
			wantErr:        true,
		},
		{
			name: "malformed image",
			args: args{
				image: "aaa:",
			},
			wantRepository: "",
			wantTag:        "",
			wantDigest:     "",
			wantErr:        true,
		},
		{
			name: "image with sha256 hash",
			args: args{
				image: "alpine:test@sha256:22eeed9e66facc651d4344ef7d9ce912fccff5bb3969e745eed3ab953f309534",
			},
			wantRepository: "docker.io/library/alpine",
			wantTag:        "test",
			wantDigest:     "sha256:22eeed9e66facc651d4344ef7d9ce912fccff5bb3969e745eed3ab953f309534",
			wantErr:        false,
		},
		{
			name: "image with sha256 hash and no tag",
			args: args{
				image: "alpine@sha256:22eeed9e66facc651d4344ef7d9ce912fccff5bb3969e745eed3ab953f309534",
			},
			wantRepository: "docker.io/library/alpine",
			wantTag:        "",
			wantDigest:     "sha256:22eeed9e66facc651d4344ef7d9ce912fccff5bb3969e745eed3ab953f309534",
			wantErr:        false,
		},
		{
			name: "shorthand only",
			args: args{
				image: "alpine",
			},
			wantRepository: "docker.io/library/alpine",
			wantTag:        "latest",
			wantDigest:     "",
			wantErr:        false,
		},
		{
			name: "shorthand with tag",
			args: args{
				image: "alpine:v1.0.0",
			},
			wantRepository: "docker.io/library/alpine",
			wantTag:        "v1.0.0",
			wantDigest:     "",
			wantErr:        false,
		},
		{
			name: "repository without registry and tag",
			args: args{
				image: "alpine/alpine",
			},
			wantRepository: "docker.io/alpine/alpine",
			wantTag:        "latest",
			wantDigest:     "",
			wantErr:        false,
		},
		{
			name: "repository without registry",
			args: args{
				image: "alpine/alpine:2.0.0",
			},
			wantRepository: "docker.io/alpine/alpine",
			wantTag:        "2.0.0",
			wantDigest:     "",
			wantErr:        false,
		},
		{
			name: "repository with registry and without tag",
			args: args{
				image: "example.com/alpine/alpine",
			},
			wantRepository: "example.com/alpine/alpine",
			wantTag:        "latest",
			wantDigest:     "",
			wantErr:        false,
		},
		{
			name: "repository with registry and tag",
			args: args{
				image: "example.com/alpine/alpine:1",
			},
			wantRepository: "example.com/alpine/alpine",
			wantTag:        "1",
			wantDigest:     "",
			wantErr:        false,
		},
		{
			name: "repository with registry and port but without tag",
			args: args{
				image: "example.com:3000/alpine/alpine",
			},
			wantRepository: "example.com:3000/alpine/alpine",
			wantTag:        "latest",
			wantDigest:     "",
			wantErr:        false,
		},
		{
			name: "repository with registry, port and tag",
			args: args{
				image: "example.com:3000/alpine/alpine:test",
			},
			wantRepository: "example.com:3000/alpine/alpine",
			wantTag:        "test",
			wantDigest:     "",
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
			if image.Digest != tt.wantDigest {
				t.Errorf("ParseImageName() hash = %v, want %v", image.Digest, tt.wantDigest)
			}
		})
	}
}
