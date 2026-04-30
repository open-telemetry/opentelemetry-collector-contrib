// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"path"
	"strings"

	"net/url"
)

// ResolveHTTPEndpoint normalizes endpoint/url-path values for HTTP exporters.
// It allows otlp-endpoint values to include a base path prefix.
func ResolveHTTPEndpoint(endpoint, urlPath string) (string, string) {
	if endpoint == "" {
		return endpoint, urlPath
	}

	if strings.Contains(endpoint, "://") {
		parsed, err := url.Parse(endpoint)
		if err == nil && parsed.Host != "" {
			if parsed.Path == "" || parsed.Path == "/" {
				return parsed.Host, urlPath
			}
			return parsed.Host, joinURLPath(parsed.Path, urlPath)
		}
	}

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
