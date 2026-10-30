// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package semconvtest

import "fmt"

// ValidateWeaverVersion exposes validateWeaverVersion to the external test
// package. The shared Makefile's integration-test discovery only works with
// a single *integration_test.go file per module root, so the version test
// lives in semconvtest_integration_test.go instead of an internal test file.
var (
	ValidateWeaverVersion = validateWeaverVersion
	RegistryForSchemaURL  = registryForSchemaURL
)

// DefaultRegistry is the registry runLiveCheck uses when no option selects
// one. Tests assert against it instead of a literal so a Renovate pin bump
// does not break them.
var DefaultRegistry = fmt.Sprintf(defaultRegistryTemplate, defaultRegistryVersion)

// ResolvedRegistry resolves the given options and returns the registry that
// runLiveCheck would pass to Weaver. weaverOptions is unexported, so the
// external test package checks the resolution rules through this accessor.
func ResolvedRegistry(opts ...WeaverOption) (string, error) {
	options, err := resolveWeaverOptions(opts)
	if err != nil {
		return "", err
	}
	return options.registry, nil
}
