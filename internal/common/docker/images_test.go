// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
		wantErr        bool
	}{
		{
			name: "empty string",
			args: args{
				image: "",
			},
			wantRepository: "",
			wantTag:        "",
			wantErr:        true,
		},
		{
			name: "malformed image",
			args: args{
				image: "aaa:",
			},
			wantRepository: "",
			wantTag:        "",
			wantErr:        true,
		},
		{
			name: "image with sha256 hash",
			args: args{
				image: "alpine:test@sha256:00000000000000",
			},
			wantRepository: "alpine",
			wantTag:        "test",
			wantErr:        false,
		},
		{
			name: "shorthand only",
			args: args{
				image: "alpine",
			},
			wantRepository: "alpine",
			wantTag:        "latest",
			wantErr:        false,
		},
		{
			name: "shorthand with tag",
			args: args{
				image: "alpine:v1.0.0",
			},
			wantRepository: "alpine",
			wantTag:        "v1.0.0",
			wantErr:        false,
		},
		{
			name: "repository without registry and tag",
			args: args{
				image: "alpine/alpine",
			},
			wantRepository: "alpine/alpine",
			wantTag:        "latest",
			wantErr:        false,
		},
		{
			name: "repository without registry",
			args: args{
				image: "alpine/alpine:2.0.0",
			},
			wantRepository: "alpine/alpine",
			wantTag:        "2.0.0",
			wantErr:        false,
		},
		{
			name: "repository with registry and without tag",
			args: args{
				image: "example.com/alpine/alpine",
			},
			wantRepository: "example.com/alpine/alpine",
			wantTag:        "latest",
			wantErr:        false,
		},
		{
			name: "repository with registry and tag",
			args: args{
				image: "example.com/alpine/alpine:1",
			},
			wantRepository: "example.com/alpine/alpine",
			wantTag:        "1",
			wantErr:        false,
		},
		{
			name: "repository with registry and port but without tag",
			args: args{
				image: "example.com:3000/alpine/alpine",
			},
			wantRepository: "example.com:3000/alpine/alpine",
			wantTag:        "latest",
			wantErr:        false,
		},
		{
			name: "repository with registry, port and tag",
			args: args{
				image: "example.com:3000/alpine/alpine:test",
			},
			wantRepository: "example.com:3000/alpine/alpine",
			wantTag:        "test",
			wantErr:        false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository, tag, err := ParseImageName(tt.args.image)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseImageName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if repository != tt.wantRepository {
				t.Errorf("ParseImageName() repository = %v, want %v", repository, tt.wantRepository)
			}
			if tag != tt.wantTag {
				t.Errorf("ParseImageName() tag = %v, want %v", tag, tt.wantTag)
			}
		})
	}
}
