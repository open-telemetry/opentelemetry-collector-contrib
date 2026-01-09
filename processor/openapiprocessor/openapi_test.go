// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package openapiprocessor

import (
	"regexp"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseOpenAPIFile(t *testing.T) {
	tests := []struct {
		name      string
		filePath  string
		wantErr   bool
		wantTitle string
		wantPaths int
	}{
		{
			name:      "valid YAML file",
			filePath:  "testdata/openapi.yaml",
			wantErr:   false,
			wantTitle: "Pet Store API",
			wantPaths: 5,
		},
		{
			name:      "valid JSON file",
			filePath:  "testdata/openapi.json",
			wantErr:   false,
			wantTitle: "Pet Store API",
			wantPaths: 5,
		},
		{
			name:     "non-existent file",
			filePath: "testdata/nonexistent.yaml",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, err := ParseOpenAPIFile(tt.filePath)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantTitle, spec.GetTitle())
			assert.Len(t, spec.GetPaths(), tt.wantPaths)
		})
	}
}

func TestOpenAPISpecMethods(t *testing.T) {
	spec, err := ParseOpenAPIFile("testdata/openapi.yaml")
	require.NoError(t, err)

	t.Run("GetTitle", func(t *testing.T) {
		assert.Equal(t, "Pet Store API", spec.GetTitle())
	})

	t.Run("GetVersion", func(t *testing.T) {
		assert.Equal(t, "1.0.0", spec.GetVersion())
	})

	t.Run("GetDescription", func(t *testing.T) {
		assert.Contains(t, spec.GetDescription(), "sample API")
	})

	t.Run("GetServers", func(t *testing.T) {
		servers := spec.GetServers()
		assert.NotEmpty(t, servers)
		assert.Contains(t, servers[0], "api.example.com")
		// Check that we have multiple servers (including versioned one)
		assert.GreaterOrEqual(t, len(servers), 2)
	})

	t.Run("GetPathTemplates", func(t *testing.T) {
		paths := spec.GetPathTemplates()
		assert.Len(t, paths, 5)
		assert.Contains(t, paths, "/users")
		assert.Contains(t, paths, "/users/{userId}")
	})

	t.Run("GetServerBasePath", func(t *testing.T) {
		basePath := spec.GetServerBasePath()
		assert.Equal(t, "/v1", basePath)
	})
}

func TestMatchPath(t *testing.T) {
	spec, err := ParseOpenAPIFile("testdata/openapi.yaml")
	require.NoError(t, err)

	tests := []struct {
		name     string
		urlPath  string
		expected string
	}{
		{
			name:     "exact match",
			urlPath:  "/users",
			expected: "/users",
		},
		{
			name:     "single parameter",
			urlPath:  "/users/123",
			expected: "/users/{userId}",
		},
		{
			name:     "multiple parameters",
			urlPath:  "/users/123/orders/456",
			expected: "/users/{userId}/orders/{orderId}",
		},
		{
			name:     "no match",
			urlPath:  "/unknown/path",
			expected: "",
		},
		{
			name:     "health endpoint",
			urlPath:  "/health",
			expected: "/health",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := spec.MatchPath(tt.urlPath)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMatchesPathTemplate(t *testing.T) {
	tests := []struct {
		name     string
		urlPath  string
		template string
		expected bool
	}{
		{
			name:     "exact match",
			urlPath:  "/users",
			template: "/users",
			expected: true,
		},
		{
			name:     "single parameter match",
			urlPath:  "/users/123",
			template: "/users/{userId}",
			expected: true,
		},
		{
			name:     "multiple parameters match",
			urlPath:  "/users/123/orders/456",
			template: "/users/{userId}/orders/{orderId}",
			expected: true,
		},
		{
			name:     "different path length",
			urlPath:  "/users/123/extra",
			template: "/users/{userId}",
			expected: false,
		},
		{
			name:     "non-matching literal",
			urlPath:  "/products/123",
			template: "/users/{userId}",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesPathTemplate(tt.urlPath, tt.template)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSplitPath(t *testing.T) {
	tests := []struct {
		path     string
		expected []string
	}{
		{
			path:     "/users",
			expected: []string{"users"},
		},
		{
			path:     "/users/123",
			expected: []string{"users", "123"},
		},
		{
			path:     "/api/v1/users",
			expected: []string{"api", "v1", "users"},
		},
		{
			path:     "/",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := splitPath(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMatchServerURL(t *testing.T) {
	spec, err := ParseOpenAPIFile("testdata/openapi.yaml")
	require.NoError(t, err)

	tests := []struct {
		name           string
		fullURL        string
		expectedMatch  bool
		expectedServer string
		expectedPath   string
	}{
		{
			name:           "match production server",
			fullURL:        "https://api.example.com/v1/users/123",
			expectedMatch:  true,
			expectedServer: "https://api.example.com/v1",
			expectedPath:   "/users/123",
		},
		{
			name:           "match staging server",
			fullURL:        "https://staging.example.com/v1/users",
			expectedMatch:  true,
			expectedServer: "https://staging.example.com/v1",
			expectedPath:   "/users",
		},
		{
			name:           "match versioned server with v2",
			fullURL:        "https://api.example.com/v2/users",
			expectedMatch:  true,
			expectedServer: "https://api.example.com/{version}",
			expectedPath:   "/users",
		},
		{
			name:           "match versioned server with v3",
			fullURL:        "https://api.example.com/v3/health",
			expectedMatch:  true,
			expectedServer: "https://api.example.com/{version}",
			expectedPath:   "/health",
		},
		{
			name:           "no match - different domain",
			fullURL:        "https://other-api.com/v1/users",
			expectedMatch:  false,
			expectedServer: "",
			expectedPath:   "",
		},
		{
			name:           "no match - invalid version",
			fullURL:        "https://api.example.com/v4/users",
			expectedMatch:  false,
			expectedServer: "",
			expectedPath:   "",
		},
		{
			name:           "match with query params",
			fullURL:        "https://api.example.com/v1/users?page=1",
			expectedMatch:  true,
			expectedServer: "https://api.example.com/v1",
			expectedPath:   "/users?page=1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matchedServer, remainingPath, matched := spec.MatchServerURL(tt.fullURL)
			assert.Equal(t, tt.expectedMatch, matched)
			if tt.expectedMatch {
				assert.Equal(t, tt.expectedServer, matchedServer)
				assert.Equal(t, tt.expectedPath, remainingPath)
			}
		})
	}
}

func TestBuildServerURLRegex(t *testing.T) {
	tests := []struct {
		name        string
		serverURL   string
		variables   map[string]*openapi3.ServerVariable
		testURL     string
		shouldMatch bool
	}{
		{
			name:        "simple URL",
			serverURL:   "https://api.example.com",
			variables:   nil,
			testURL:     "https://api.example.com/users",
			shouldMatch: true,
		},
		{
			name:        "URL with path",
			serverURL:   "https://api.example.com/v1",
			variables:   nil,
			testURL:     "https://api.example.com/v1/users",
			shouldMatch: true,
		},
		{
			name:      "URL with variable enum",
			serverURL: "https://api.example.com/{version}",
			variables: map[string]*openapi3.ServerVariable{
				"version": {
					Default: "v1",
					Enum:    []string{"v1", "v2"},
				},
			},
			testURL:     "https://api.example.com/v2/users",
			shouldMatch: true,
		},
		{
			name:      "URL with variable enum - no match",
			serverURL: "https://api.example.com/{version}",
			variables: map[string]*openapi3.ServerVariable{
				"version": {
					Default: "v1",
					Enum:    []string{"v1", "v2"},
				},
			},
			testURL:     "https://api.example.com/v3/users",
			shouldMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern := buildServerURLRegex(tt.serverURL, tt.variables)
			assert.NotEmpty(t, pattern)

			re, err := regexp.Compile(pattern)
			require.NoError(t, err)

			matched := re.MatchString(tt.testURL)
			assert.Equal(t, tt.shouldMatch, matched)
		})
	}
}

func TestIsHTTPURL(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"http://example.com/openapi.yaml", true},
		{"https://example.com/openapi.yaml", true},
		{"https://api.example.com:8080/v1/openapi.json", true},
		{"/etc/otel/openapi.yaml", false},
		{"./openapi.yaml", false},
		{"openapi.yaml", false},
		{"file:///etc/otel/openapi.yaml", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := isHTTPURL(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}
