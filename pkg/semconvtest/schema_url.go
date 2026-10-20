// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package semconvtest // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/semconvtest"

import (
	"fmt"
	"strings"

	"golang.org/x/mod/semver"
)

const (
	schemaURLPrefix     = "https://opentelemetry.io/schemas/"
	weaverRegistryBase  = "https://github.com/open-telemetry/semantic-conventions"
	weaverModelSelector = "[model]"
)

// normalizeSchemaURL takes an OpenTelemetry Schema URL (e.g. "https://opentelemetry.io/schemas/1.27.0")
// and translates it to Weaver's virtual directory registry string
// (e.g. "https://github.com/open-telemetry/semantic-conventions@v1.27.0[model]").
func normalizeSchemaURL(schemaURL string) (string, error) {
	if schemaURL == "" {
		return "", fmt.Errorf("schema URL must not be empty")
	}

	if !strings.HasPrefix(schemaURL, schemaURLPrefix) {
		return "", fmt.Errorf("unsupported schema URL %q: expected prefix %q", schemaURL, schemaURLPrefix)
	}

	version := strings.TrimPrefix(schemaURL, schemaURLPrefix)
	version = strings.TrimSuffix(version, "/")
	version = strings.TrimPrefix(version, "v")

	semverVersion := "v" + version
	if !semver.IsValid(semverVersion) {
		return "", fmt.Errorf("invalid semver version %q extracted from schema URL %q", version, schemaURL)
	}

	return fmt.Sprintf("%s@%s%s", weaverRegistryBase, semverVersion, weaverModelSelector), nil
}
