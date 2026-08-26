// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package noop provides a no-operation lookup source for testing.
package noop // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor/internal/source/noop"

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor/lookupsource"
)

const sourceType = "noop"

type Config struct{}

func (*Config) Validate() error {
	return nil
}

func NewFactory() lookupsource.SourceFactory {
	return lookupsource.NewSourceFactory(
		sourceType,
		createDefaultConfig,
		createSource,
	)
}

func createDefaultConfig() lookupsource.SourceConfig {
	return &Config{}
}

func createSource(
	_ context.Context,
	_ lookupsource.CreateSettings,
	_ lookupsource.SourceConfig,
) (lookupsource.Source, error) {
	return lookupsource.NewSource(
		noopLookup,
		func() string { return sourceType },
		nil, // no start needed
		nil, // no shutdown needed
	), nil
}

// noopLookup always returns not found.
func noopLookup(_ context.Context, _ string) (any, bool, error) {
	return nil, false, nil
}
