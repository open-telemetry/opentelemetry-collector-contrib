// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package subprocessmanager

import (
	"context"
	"reflect"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestFormatEnvSlice(t *testing.T) {
	var formatEnvSliceTests = []struct {
		name     string
		envSlice *[]EnvConfig
		want     []string
		wantNil  bool
	}{
		{
			name:     "empty slice",
			envSlice: &[]EnvConfig{},
			want:     nil,
			wantNil:  true,
		},
		{
			name: "one entry",
			envSlice: &[]EnvConfig{
				{
					Name:  "DATA_SOURCE",
					Value: "password:username",
				},
			},
			want: []string{
				"DATA_SOURCE=password:username",
			},
			wantNil: false,
		},
		{
			name: "three entries",
			envSlice: &[]EnvConfig{
				{
					Name:  "DATA_SOURCE",
					Value: "password:username",
				},
				{
					Name:  "",
					Value: "",
				},
				{
					Name:  "john",
					Value: "doe",
				},
			},
			want: []string{
				"DATA_SOURCE=password:username",
				"=",
				"john=doe",
			},
			wantNil: false,
		},
	}

	for _, test := range formatEnvSliceTests {
		t.Run(test.name, func(t *testing.T) {
			got := formatEnvSlice(test.envSlice)
			if test.wantNil && got != nil {
				t.Errorf("formatEnvSlice() got = %v, wantNil %v", got, test.wantNil)
				return
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("formatEnvSlice() got = %v, want %v", got, test.want)
			}
		})
	}
}

func TestRun(t *testing.T) {
	var runTests = []struct {
		name        string
		process     *SubprocessConfig
		wantElapsed time.Duration
		wantErr     bool
	}{
		{
			name: "normal process 1, error process exit",
			process: &SubprocessConfig{
				Command: "go run testdata/test_crasher.go",
				Env: []EnvConfig{
					{
						Name:  "DATA_SOURCE",
						Value: "username:password@(url:port)/dbname",
					},
				},
			},
			wantElapsed: 4 * time.Millisecond,
			wantErr:     false,
		},
		{
			name: "normal process 2, normal process exit",
			process: &SubprocessConfig{
				Command: "go version",
				Env: []EnvConfig{
					{
						Name:  "DATA_SOURCE",
						Value: "username:password@(url:port)/dbname",
					},
				},
			},
			wantElapsed: 0 * time.Nanosecond,
			wantErr:     false,
		},
		{
			name: "shellquote error",
			process: &SubprocessConfig{
				Command: "command flag='something",
				Env:     []EnvConfig{},
			},
			wantElapsed: 0,
			wantErr:     true,
		},
	}

	for _, test := range runTests {
		t.Run(test.name, func(t *testing.T) {
			logger, _ := zap.NewProduction()
			got, err := test.process.Run(context.Background(), logger)
			if test.wantErr && err == nil {
				t.Errorf("Run() got = %v, wantErr %v", got, test.wantErr)
				return
			}
			if got < test.wantElapsed {
				t.Errorf("Run() got = %v, want larger than %v", got, test.wantElapsed)
			}
		})
	}
}
