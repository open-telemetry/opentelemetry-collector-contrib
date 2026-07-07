// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !aix

package pebbletailstorageextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/tailstorage/pebbletailstorageextension"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type pebbleTailStorageExtension struct {
	settings extension.Settings
	cfg      *Config

	storage *storage
}

var _ extension.Extension = (*pebbleTailStorageExtension)(nil)

func newExtension(settings extension.Settings, cfg *Config) *pebbleTailStorageExtension {
	return &pebbleTailStorageExtension{
		settings: settings,
		cfg:      cfg,
	}
}

func (e *pebbleTailStorageExtension) Start(ctx context.Context, _ component.Host) error {
	storage, err := newStorage(ctx, e.cfg.Directory, e.settings.Logger)
	if err != nil {
		return err
	}
	e.storage = storage
	return nil
}

func (e *pebbleTailStorageExtension) Shutdown(_ context.Context) error {
	if e.storage == nil {
		return nil
	}
	err := e.storage.Close()
	e.storage = nil
	return err
}

func (e *pebbleTailStorageExtension) Append(traceID pcommon.TraceID, td ptrace.Traces) error {
	return e.storage.Append(traceID, td)
}

func (e *pebbleTailStorageExtension) Take(traceID pcommon.TraceID) (ptrace.Traces, error) {
	return e.storage.Take(traceID)
}

func (e *pebbleTailStorageExtension) Delete(traceID pcommon.TraceID) error {
	return e.storage.Delete(traceID)
}
