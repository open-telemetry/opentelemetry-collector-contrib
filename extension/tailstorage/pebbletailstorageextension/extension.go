// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pebbletailstorageextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/tailstorage/pebbletailstorageextension"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
)

type pebbleTailStorageExtension struct {
	settings extension.Settings
	cfg      *Config
}

var _ extension.Extension = (*pebbleTailStorageExtension)(nil)

func newExtension(settings extension.Settings, cfg *Config) *pebbleTailStorageExtension {
	return &pebbleTailStorageExtension{
		settings: settings,
		cfg:      cfg,
	}
}

func (*pebbleTailStorageExtension) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (*pebbleTailStorageExtension) Shutdown(_ context.Context) error {
	return nil
}
