// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEndpointHasScheme(t *testing.T) {
	assert.True(t, EndpointHasScheme("https://mydom.com/my-collector"))
	assert.True(t, EndpointHasScheme("http://collector:4318"))
	assert.False(t, EndpointHasScheme("collector:4318"))
	assert.False(t, EndpointHasScheme("mydom.com/my-collector"))
}

func TestHTTPURLPath(t *testing.T) {
	tests := []struct {
		name         string
		endpoint     string
		urlPath      string
		expectedPath string
	}{
		{
			name:         "scheme and base path are joined",
			endpoint:     "https://mydom.com/my-collector",
			urlPath:      "/v1/logs",
			expectedPath: "/my-collector/v1/logs",
		},
		{
			name:         "custom root path combines with explicit urlPath",
			endpoint:     "https://mydom.com/my-collector",
			urlPath:      "/custom/path",
			expectedPath: "/my-collector/custom/path",
		},
		{
			name:         "endpoint without base path keeps urlPath",
			endpoint:     "https://mydom.com",
			urlPath:      "/v1/logs",
			expectedPath: "/v1/logs",
		},
		{
			name:         "root endpoint path keeps urlPath",
			endpoint:     "https://mydom.com/",
			urlPath:      "/v1/logs",
			expectedPath: "/v1/logs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedPath, HTTPURLPath(tt.endpoint, tt.urlPath))
		})
	}
}

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
			name:             "host and base path without scheme are split",
			endpoint:         "mydom.com/my-collector",
			urlPath:          "/v1/metrics",
			expectedEndpoint: "mydom.com",
			expectedPath:     "/my-collector/v1/metrics",
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
