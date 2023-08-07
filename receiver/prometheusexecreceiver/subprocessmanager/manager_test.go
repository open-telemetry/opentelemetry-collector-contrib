// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package subprocessmanager

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestFormatEnvSlice(t *testing.T) {
	var formatEnvSliceTests = []struct {
		name     string
		envSlice *[]EnvConfig
		want     []string
	}{
		{
			name:     "empty slice",
			envSlice: &[]EnvConfig{},
			want:     nil,
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
		},
	}

	for _, test := range formatEnvSliceTests {
		t.Run(test.name, func(t *testing.T) {
			got := formatEnvSlice(test.envSlice)
			assert.Equal(t, test.want, got)
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
			name: "sleep 4ms and succeeds",
			process: &SubprocessConfig{
				Command: "go run testdata/test_crasher.go 4 0",
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
			name: "sleep 4ms and fail",
			process: &SubprocessConfig{
				Command: "go run testdata/test_crasher.go 4 1",
				Env:     []EnvConfig{},
			},
			wantElapsed: 4 * time.Millisecond,
			wantErr:     true,
		},
		{
			// Run test_crasher with 2 arguments:
			// - sleepTime: "1 2 3" (with spaces in it, because surrounded by quotes), which is invalid
			// - exitCode: 0
			//
			// test_crasher will see that "1 2 3" is not a proper integer and will
			// default to 2ms, and its exit code will be zero (the second argument)
			//
			// If the command line is not parsed properly (notably on Windows),
			// "1 2 3" will be considered as 3 separate arguments, so test_crasher
			// will exit with code 2 (the wrongly 2nd argument)
			// The below test will detect and report this issue.
			name: "proper parsing of arguments with spaces, surrounded by quotes",
			process: &SubprocessConfig{
				Command: "go run testdata/test_crasher.go \"1 2 3\" 0",
				Env:     []EnvConfig{},
			},
			wantElapsed: 2 * time.Millisecond,
			wantErr:     false,
		},
		{
			name: "sleep 4 and succeeds, with quotes",
			process: &SubprocessConfig{
				Command: "\"go\" \"run\" \"testdata/test_crasher.go\" \"4\" \"0\"",
				Env:     []EnvConfig{},
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
			if test.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Less(t, test.wantElapsed, got)
		})
	}
}
