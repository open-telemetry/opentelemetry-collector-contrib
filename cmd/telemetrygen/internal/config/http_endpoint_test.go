// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config

import "testing"

import "github.com/stretchr/testify/assert"

func TestResolveHTTPEndpoint(t *testing.T) {
	tests := []struct {
		name             string
		endpoint         string
		urlPath          string
		expectedEndpoint string
		expectedPath     string
	}{
		{
			name:             "host only endpoint remains unchanged",
			endpoint:         "collector:4318",
			urlPath:          "/v1/logs",
			expectedEndpoint: "collector:4318",
			expectedPath:     "/v1/logs",
		},
		{
			name:             "scheme and base path are split",
			endpoint:         "https://mydom.com/my-collector",
			urlPath:          "/v1/logs",
			expectedEndpoint: "mydom.com",
			expectedPath:     "/my-collector/v1/logs",
		},
		{
			name:             "host and base path without scheme are split",
			endpoint:         "mydom.com/my-collector",
			urlPath:          "/v1/metrics",
			expectedEndpoint: "mydom.com",
			expectedPath:     "/my-collector/v1/metrics",
		},
		{
			name:             "custom root path combines with explicit urlPath",
			endpoint:         "https://mydom.com/my-collector",
			urlPath:          "/custom/path",
			expectedEndpoint: "mydom.com",
			expectedPath:     "/my-collector/custom/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoint, urlPath := ResolveHTTPEndpoint(tt.endpoint, tt.urlPath)
			assert.Equal(t, tt.expectedEndpoint, endpoint)
			assert.Equal(t, tt.expectedPath, urlPath)
		})
	}
}
