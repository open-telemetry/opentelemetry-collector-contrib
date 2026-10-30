// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build aix || freebsd || solaris

package datadogextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/metadata"
)

func NewFactory() extension.Factory {
	return extension.NewFactory(
		metadata.Type,
		func() component.Config {
			return nil
		},
		createUnsupported,
		metadata.ExtensionStability,
	)
}

func createUnsupported(context.Context, extension.Settings, component.Config) (extension.Extension, error) {
	return nil, errors.New("datadogextension is not supported on aix, freebsd, or solaris")
}
