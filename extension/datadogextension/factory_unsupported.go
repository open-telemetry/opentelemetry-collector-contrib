// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build aix || solaris

package datadogextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/metadata"
)

// factory exists only so Option has the same shape here as on supported platforms.
type factory struct{}

// Option configures the Datadog extension factory. WithConfigOptions is absent here:
// its parameter type lives in pkg/datadog/agentcomponents, which does not build on
// these platforms.
type Option func(*factory)

func NewFactory() extension.Factory {
	return extension.NewFactory(
		metadata.Type,
		func() component.Config {
			return nil
		},
		createAix,
		metadata.ExtensionStability,
	)
}

// NewFactoryWithOptions creates a factory for the Datadog extension. Options are
// ignored: the extension is unsupported on this platform.
func NewFactoryWithOptions(...Option) extension.Factory {
	return NewFactory()
}

func createAix(context.Context, extension.Settings, component.Config) (extension.Extension, error) {
	return nil, errors.New("datadogextension is not supported on aix or solaris")
}
