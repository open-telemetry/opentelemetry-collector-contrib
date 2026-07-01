// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"net/url"
	"path"
	"strings"
)

// EndpointHasScheme reports whether endpoint can be passed to WithEndpointURL.
func EndpointHasScheme(endpoint string) bool {
	if strings.Contains(endpoint, "://") {
		parsed, err := url.Parse(endpoint)
		return err == nil && parsed.Host != ""
	}

	return false
}

// HTTPURLPath combines a URL endpoint's base path with the signal URL path.
func HTTPURLPath(endpoint, urlPath string) string {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" || parsed.Path == "" || parsed.Path == "/" {
		return urlPath
	}

	return joinURLPath(parsed.Path, urlPath)
}

// ResolveHTTPEndpoint normalizes endpoint/url-path values for HTTP exporters
// when the endpoint is in host/path form without a URL scheme.
func ResolveHTTPEndpoint(endpoint, urlPath string) (string, string) {
	slash := strings.Index(endpoint, "/")
	if slash > 0 {
		basePath := endpoint[slash:]
		return endpoint[:slash], joinURLPath(basePath, urlPath)
	}

	return endpoint, urlPath
}

func joinURLPath(basePath, urlPath string) string {
	trimmedBase := strings.TrimPrefix(basePath, "/")
	trimmedPath := strings.TrimPrefix(urlPath, "/")

	if trimmedPath == "" {
		return "/" + trimmedBase
	}

	return "/" + path.Join(trimmedBase, trimmedPath)
}
