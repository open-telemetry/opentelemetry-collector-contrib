// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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

func (e *pebbleTailStorageExtension) Start(_ context.Context, _ component.Host) error {
	storage, err := newStorage(e.cfg.Directory, e.settings.Logger)
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

func (e *pebbleTailStorageExtension) Append(traceID pcommon.TraceID, rss ptrace.ResourceSpans) {
	e.storage.Append(traceID, rss)
}

func (e *pebbleTailStorageExtension) Take(traceID pcommon.TraceID) (ptrace.Traces, bool) {
	return e.storage.Take(traceID)
}

func (e *pebbleTailStorageExtension) Delete(traceID pcommon.TraceID) {
	e.storage.Delete(traceID)
}
