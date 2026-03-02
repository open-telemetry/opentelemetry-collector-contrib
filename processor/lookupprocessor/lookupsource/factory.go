// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lookupsource // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor/lookupsource"

import (
	"context"

	"go.opentelemetry.io/collector/component"
)

type SourceConfig interface {
	Validate() error
}

type CreateSettings struct {
	TelemetrySettings component.TelemetrySettings
	BuildInfo         component.BuildInfo
}

type CreateSourceFunc func(ctx context.Context, settings CreateSettings, cfg SourceConfig) (Source, error)

type CreateDefaultConfigFunc func() SourceConfig

// SourceFactory creates Source instances from configuration.
//
// Use [NewSourceFactory] to create implementations.
type SourceFactory interface {
	Type() string

	CreateDefaultConfig() SourceConfig

	CreateSource(ctx context.Context, settings CreateSettings, cfg SourceConfig) (Source, error)
}

// NewSourceFactory creates a new SourceFactory.
//
// Parameters:
//   - typ: The source type identifier (e.g., "yaml", "http").
//   - createDefaultConfig: Returns the default configuration for this source.
//   - createSource: Creates a Source instance from configuration.
//
// Example:
//
//	func NewHTTPSourceFactory() lookupsource.SourceFactory {
//	    return lookupsource.NewSourceFactory(
//	        "http",
//	        func() lookupsource.SourceConfig { return &HTTPConfig{Timeout: 5 * time.Second} },
//	        createHTTPSource,
//	    )
//	}
func NewSourceFactory(
	typ string,
	createDefaultConfig CreateDefaultConfigFunc,
	createSource CreateSourceFunc,
) SourceFactory {
	return &sourceFactoryImpl{
		typ:                 typ,
		createDefaultConfig: createDefaultConfig,
		createSource:        createSource,
	}
}

type sourceFactoryImpl struct {
	typ                 string
	createDefaultConfig CreateDefaultConfigFunc
	createSource        CreateSourceFunc
}

func (f *sourceFactoryImpl) Type() string {
	return f.typ
}

func (f *sourceFactoryImpl) CreateDefaultConfig() SourceConfig {
	if f.createDefaultConfig == nil {
		return nil
	}
	return f.createDefaultConfig()
}

func (f *sourceFactoryImpl) CreateSource(ctx context.Context, settings CreateSettings, cfg SourceConfig) (Source, error) {
	if f.createSource == nil {
		return nil, nil
	}
	return f.createSource(ctx, settings, cfg)
}
