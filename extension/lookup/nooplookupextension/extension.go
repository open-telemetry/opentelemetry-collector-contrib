// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package nooplookupextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/lookup/nooplookupextension"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/lookup"
)

const typeStr = "nooplookup"

var Type = component.MustNewType(typeStr)

type Config struct{}

var _ component.Config = (*Config)(nil)

func (*Config) Validate() error { return nil }

func NewFactory() extension.Factory {
	return extension.NewFactory(
		Type,
		createDefaultConfig,
		createExtension,
		component.StabilityLevelDevelopment,
	)
}

func NewLookupExtensionFactory() extension.Factory {
	return NewFactory()
}

func createDefaultConfig() component.Config {
	return &Config{}
}

func createExtension(_ context.Context, set extension.Settings, _ component.Config) (extension.Extension, error) {
	return NewLookupExtension(set.TelemetrySettings)
}

// NewLookupExtension creates the noop lookup extension using functional composition.
func NewLookupExtension(settings component.TelemetrySettings) (lookup.LookupExtension, error) {
	base, err := lookup.NewBaseSource(typeStr, settings, nil)
	if err != nil {
		return nil, err
	}

	return lookup.NewLookupExtension(
		base.WrapLookup(func(context.Context, string) (any, bool, error) {
			return nil, false, nil
		}),
		base.TypeFunc(),
		lookup.StartFunc(base.Start),
		lookup.ShutdownFunc(base.Shutdown),
	), nil
}
