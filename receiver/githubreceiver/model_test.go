// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "converts underscores to hyphens",
			input:    "hello_world",
			expected: "hello-world",
		},
		{
			name:     "converts to lowercase",
			input:    "HELLO_WORLD",
			expected: "hello-world",
		},
		{
			name:     "handles mixed case and multiple underscores",
			input:    "Hello_Big_WORLD",
			expected: "hello-big-world",
		},
		{
			name:     "handles string with no underscores",
			input:    "HelloWorld",
			expected: "helloworld",
		},
		{
			name:     "handles empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatString(tt.input)
			if got != tt.expected {
				t.Errorf("formatString() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSplitRefWorkflowPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "simple workflow with version",
			input:    "my-great-workflow@v1.0.0",
			expected: "my-great-workflow",
			wantErr:  false,
		},
		{
			name:     "workflow with SHA",
			input:    "my-great-workflow@3421498310493281409328140932840192384",
			expected: "my-great-workflow",
			wantErr:  false,
		},
		{
			name:     "full path workflow",
			input:    "org/repo/.github/my-file-path/with/folder/build-woot.yaml@v0.2.3",
			expected: "build-woot",
			wantErr:  false,
		},
		{
			name:     "uppercase file",
			input:    "org/repo/.github/my-file-path/with/folder/BUILD-WOOT.yaml@v0.2.3",
			expected: "build-woot",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := splitRefWorkflowPath(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestReplaceAPIURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "converts api.github.com URL to html URL",
			input:    "https://api.github.com/repos/open-telemetry/opentelemetry-collector-contrib/pull/1234",
			expected: "https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/1234",
		},
		{
			name:     "converts api.github.com workflow URL to html URL",
			input:    "https://api.github.com/repos/open-telemetry/opentelemetry-collector-contrib/actions/runs/1234",
			expected: "https://github.com/open-telemetry/opentelemetry-collector-contrib/actions/runs/1234",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := replaceAPIURL(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
