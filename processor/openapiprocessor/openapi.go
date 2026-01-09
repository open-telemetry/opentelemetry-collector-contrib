// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package openapiprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/openapiprocessor"

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
)

// OpenAPISpec wraps the kin-openapi document with helper methods.
type OpenAPISpec struct {
	doc            *openapi3.T
	serverMatchers []serverMatcher
}

// serverMatcher holds a compiled regex for matching server URLs.
type serverMatcher struct {
	regex       *regexp.Regexp
	originalURL string
	// basePath is the path portion of the server URL (e.g., "/api/v1")
	basePath string
}

// isHTTPURL checks if the given string is an HTTP or HTTPS URL.
func isHTTPURL(path string) bool {
	return strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://")
}

// fetchOpenAPIFromURL fetches an OpenAPI spec from an HTTP/HTTPS URL.
func fetchOpenAPIFromURL(specURL string, timeout time.Duration) ([]byte, error) {
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	client := &http.Client{
		Timeout: timeout,
	}

	resp, err := client.Get(specURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch OpenAPI spec from %s: %w", specURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch OpenAPI spec from %s: HTTP %d", specURL, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read OpenAPI spec from %s: %w", specURL, err)
	}

	return body, nil
}

// ParseOpenAPIFile reads and parses an OpenAPI specification file using kin-openapi.
// It supports both YAML and JSON formats.
// The filePath can be a local file path or an HTTP/HTTPS URL.
func ParseOpenAPIFile(filePath string) (*OpenAPISpec, error) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	var doc *openapi3.T
	var err error

	if isHTTPURL(filePath) {
		// Fetch from URL
		data, fetchErr := fetchOpenAPIFromURL(filePath, 30*time.Second)
		if fetchErr != nil {
			return nil, fetchErr
		}

		// Parse the URL to use as base for resolving references
		baseURL, _ := url.Parse(filePath)
		doc, err = loader.LoadFromDataWithPath(data, baseURL)
	} else {
		// Load from file
		doc, err = loader.LoadFromFile(filePath)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load OpenAPI file %s: %w", filePath, err)
	}

	// Validate the document
	ctx := context.Background()
	if err := doc.Validate(ctx); err != nil {
		return nil, fmt.Errorf("invalid OpenAPI document: %w", err)
	}

	// Validate minimum required fields
	if doc.Paths == nil || doc.Paths.Len() == 0 {
		return nil, fmt.Errorf("OpenAPI file must contain at least one path")
	}

	spec := &OpenAPISpec{doc: doc}
	spec.serverMatchers = spec.buildServerMatchers()

	return spec, nil
}

// GetTitle returns the API title from the info section.
func (s *OpenAPISpec) GetTitle() string {
	if s.doc.Info != nil {
		return s.doc.Info.Title
	}
	return ""
}

// GetVersion returns the API version from the info section.
func (s *OpenAPISpec) GetVersion() string {
	if s.doc.Info != nil {
		return s.doc.Info.Version
	}
	return ""
}

// GetDescription returns the API description from the info section.
func (s *OpenAPISpec) GetDescription() string {
	if s.doc.Info != nil {
		return s.doc.Info.Description
	}
	return ""
}

// GetPaths returns a map of all paths in the specification.
func (s *OpenAPISpec) GetPaths() map[string]*openapi3.PathItem {
	if s.doc.Paths == nil {
		return nil
	}
	return s.doc.Paths.Map()
}

// GetServers returns a list of server URLs from the specification.
func (s *OpenAPISpec) GetServers() []string {
	var servers []string
	for _, server := range s.doc.Servers {
		servers = append(servers, server.URL)
	}
	return servers
}

// buildServerMatchers creates regex matchers for all server URLs in the spec.
// Server URLs can contain variables like {version} which are converted to regex patterns.
func (s *OpenAPISpec) buildServerMatchers() []serverMatcher {
	var matchers []serverMatcher

	for _, server := range s.doc.Servers {
		serverURL := server.URL
		if serverURL == "" {
			continue
		}

		// Parse the server URL to extract the base path
		basePath := ""
		if parsed, err := url.Parse(serverURL); err == nil {
			basePath = strings.TrimSuffix(parsed.Path, "/")
		}

		// Build regex pattern from server URL
		// Handle server variables like {version} or {basePath}
		pattern := buildServerURLRegex(serverURL, server.Variables)
		if pattern == "" {
			continue
		}

		compiled, err := regexp.Compile(pattern)
		if err != nil {
			continue
		}

		matchers = append(matchers, serverMatcher{
			regex:       compiled,
			originalURL: serverURL,
			basePath:    basePath,
		})
	}

	return matchers
}

// buildServerURLRegex converts a server URL (potentially with variables) to a regex pattern.
// For example: "https://api.example.com/{version}" -> "^https://api\\.example\\.com/[^/]+"
func buildServerURLRegex(serverURL string, variables map[string]*openapi3.ServerVariable) string {
	// Escape regex special characters
	escaped := regexp.QuoteMeta(serverURL)

	// Replace escaped server variables {var} -> \{var\} with appropriate patterns
	varRegex := regexp.MustCompile(`\\\{([^}]+)\\\}`)
	pattern := varRegex.ReplaceAllStringFunc(escaped, func(match string) string {
		// Extract variable name from the escaped match
		varName := match[2 : len(match)-2] // Remove \{ and \}

		// Check if this variable has an enum (allowed values)
		if variables != nil {
			if serverVar, ok := variables[varName]; ok && len(serverVar.Enum) > 0 {
				// Create alternation pattern from enum values
				var escapedEnums []string
				for _, enumVal := range serverVar.Enum {
					escapedEnums = append(escapedEnums, regexp.QuoteMeta(enumVal))
				}
				return "(" + strings.Join(escapedEnums, "|") + ")"
			}
		}

		// Default: match any non-slash characters
		return "[^/]+"
	})

	// Remove trailing slash if present for matching flexibility
	pattern = strings.TrimSuffix(pattern, "/")
	pattern = strings.TrimSuffix(pattern, "\\/")

	return "^" + pattern
}

// MatchServerURL checks if a URL matches any of the server URLs in the spec.
// Returns the matched server info and the remaining path after the server URL.
// If no match is found, returns empty strings.
func (s *OpenAPISpec) MatchServerURL(fullURL string) (matchedServer string, remainingPath string, matched bool) {
	for _, matcher := range s.serverMatchers {
		if loc := matcher.regex.FindStringIndex(fullURL); loc != nil {
			// Extract the remaining path after the matched server URL
			remaining := fullURL[loc[1]:]

			// Ensure the remaining path starts with / or is empty
			if remaining == "" || remaining[0] == '/' || remaining[0] == '?' {
				return matcher.originalURL, remaining, true
			}
		}
	}
	return "", "", false
}

// GetServerBasePath returns the base path from the first server URL.
// This is useful when server URLs include a base path like /api/v1.
func (s *OpenAPISpec) GetServerBasePath() string {
	if len(s.serverMatchers) > 0 {
		return s.serverMatchers[0].basePath
	}
	return ""
}

// GetPathTemplates returns a list of all path templates in the specification.
func (s *OpenAPISpec) GetPathTemplates() []string {
	var paths []string
	if s.doc.Paths == nil {
		return paths
	}
	for path := range s.doc.Paths.Map() {
		paths = append(paths, path)
	}
	return paths
}

// GetOperationByPathAndMethod returns the operation for a given path and HTTP method.
func (s *OpenAPISpec) GetOperationByPathAndMethod(path, method string) *openapi3.Operation {
	if s.doc.Paths == nil {
		return nil
	}
	pathItem := s.doc.Paths.Find(path)
	if pathItem == nil {
		return nil
	}
	return pathItem.GetOperation(method)
}

// MatchPath attempts to match a URL path against the OpenAPI paths and returns
// the matching template path. Returns empty string if no match found.
func (s *OpenAPISpec) MatchPath(urlPath string) string {
	if s.doc.Paths == nil {
		return ""
	}

	// Strip query parameters and fragments
	if parsedURL, err := url.Parse(urlPath); err == nil {
		urlPath = parsedURL.Path
	}

	// Try to find a matching path using path template matching
	for pathTemplate := range s.doc.Paths.Map() {
		if matchesPathTemplate(urlPath, pathTemplate) {
			return pathTemplate
		}
	}

	return ""
}

// matchesPathTemplate checks if a URL path matches an OpenAPI path template.
// For example, "/users/123" matches "/users/{userId}".
func matchesPathTemplate(urlPath, template string) bool {
	urlParts := splitPath(urlPath)
	templateParts := splitPath(template)

	if len(urlParts) != len(templateParts) {
		return false
	}

	for i, templatePart := range templateParts {
		// Check if this part is a path parameter (e.g., {userId})
		if len(templatePart) > 2 && templatePart[0] == '{' && templatePart[len(templatePart)-1] == '}' {
			// Path parameter matches any non-empty value
			if urlParts[i] == "" {
				return false
			}
			continue
		}

		// Exact match required for non-parameter parts
		if urlParts[i] != templatePart {
			return false
		}
	}

	return true
}

// splitPath splits a URL path into parts, filtering out empty strings.
func splitPath(path string) []string {
	var parts []string
	start := 0
	for i := 0; i <= len(path); i++ {
		if i == len(path) || path[i] == '/' {
			if i > start {
				parts = append(parts, path[start:i])
			}
			start = i + 1
		}
	}
	return parts
}
