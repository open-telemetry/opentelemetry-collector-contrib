// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build aix

package pebbletailstorageextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/tailstorage/pebbletailstorageextension"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/tailstorage/pebbletailstorageextension/internal/metadata"
)

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

func createAix(context.Context, extension.Settings, component.Config) (extension.Extension, error) {
	return nil, errors.New("pebbletailstorageextension is not supported on aix")
}
