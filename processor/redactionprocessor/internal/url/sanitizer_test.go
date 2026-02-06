// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package url

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewURLSanitizer(t *testing.T) {
	tests := []struct {
		name   string
		config URLSanitizationConfig
		wantOk bool
	}{
		{
			name: "valid config with attributes",
			config: URLSanitizationConfig{
				Enabled:    true,
				Attributes: []string{"http.url", "http.target"},
			},
			wantOk: true,
		},
		{
			name: "valid config without attributes",
			config: URLSanitizationConfig{
				Enabled: true,
			},
			wantOk: true,
		},
		{
			name: "valid config disabled",
			config: URLSanitizationConfig{
				Enabled: false,
			},
			wantOk: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sanitizer, err := NewURLSanitizer(tt.config)
			if tt.wantOk {
				require.NoError(t, err)
				assert.NotNil(t, sanitizer)
				assert.NotNil(t, sanitizer.classifier)
				assert.Len(t, sanitizer.attributes, len(tt.config.Attributes))
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestURLSanitizer_SanitizeURL(t *testing.T) {
	config := URLSanitizationConfig{
		Enabled:    true,
		Attributes: []string{"http.url"},
	}
	sanitizer, err := NewURLSanitizer(config)
	require.NoError(t, err)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "url with uuid",
			input:    "/api/users/123e4567-e89b-12d3-a456-426614174000",
			expected: "/api/users/*",
		},
		{
			name:     "url with numeric id",
			input:    "/api/users/12345",
			expected: "/api/users/*",
		},
		{
			name:     "url with multiple ids",
			input:    "/api/users/123/posts/456",
			expected: "/api/users/*/posts/*",
		},
		{
			name:     "simple path",
			input:    "/api/users",
			expected: "/api/users",
		},
		{
			name:     "url with query params",
			input:    "/api/users?id=123&name=john",
			expected: "/api/users",
		},
		{
			name:     "empty url",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizer.SanitizeURL(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestURLSanitizer_SanitizeAttributeURL(t *testing.T) {
	config := URLSanitizationConfig{
		Enabled:    true,
		Attributes: []string{"http.url", "http.target"},
	}
	sanitizer, err := NewURLSanitizer(config)
	require.NoError(t, err)

	tests := []struct {
		name          string
		url           string
		attributeKey  string
		shouldProcess bool
	}{
		{
			name:          "configured attribute http.url",
			url:           "/api/users/123",
			attributeKey:  "http.url",
			shouldProcess: true,
		},
		{
			name:          "configured attribute http.target",
			url:           "/api/posts/456",
			attributeKey:  "http.target",
			shouldProcess: true,
		},
		{
			name:          "non-configured attribute",
			url:           "/api/users/123",
			attributeKey:  "http.method",
			shouldProcess: false,
		},
		{
			name:          "empty url with configured attribute",
			url:           "",
			attributeKey:  "http.url",
			shouldProcess: true,
		},
		{
			name:          "empty url with non-configured attribute",
			url:           "",
			attributeKey:  "http.method",
			shouldProcess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizer.SanitizeAttributeURL(tt.url, tt.attributeKey)

			if tt.url == "" {
				assert.Empty(t, result, "empty URL should remain empty")
				return
			}

			if tt.shouldProcess {
				assert.NotNil(t, result)
			} else {
				assert.Equal(t, tt.url, result, "URL should not be modified for non-configured attributes")
			}
		})
	}
}

func TestURLSanitizer_EmptyAttributes(t *testing.T) {
	config := URLSanitizationConfig{
		Enabled:    true,
		Attributes: []string{},
	}
	sanitizer, err := NewURLSanitizer(config)
	require.NoError(t, err)

	tests := []struct {
		name         string
		url          string
		attributeKey string
	}{
		{
			name:         "no configured attributes - http.url",
			url:          "/api/users/123",
			attributeKey: "http.url",
		},
		{
			name:         "no configured attributes - http.target",
			url:          "/api/posts/456",
			attributeKey: "http.target",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizer.SanitizeAttributeURL(tt.url, tt.attributeKey)
			// When no attributes are configured, URLs should not be sanitized
			assert.Equal(t, tt.url, result)
		})
	}
}
