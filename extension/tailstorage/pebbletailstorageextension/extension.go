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
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/tailstorage/pebbletailstorageextension/internal/metadata"
)

const (
	attrOperation = "operation"
	attrOutcome   = "outcome"

	operationAppend = "append"
	operationTake   = "take"
	operationDelete = "delete"

	outcomeSuccess = "success"
	outcomeFailure = "failure"
)

type tailStorage interface {
	Append(traceID pcommon.TraceID, td ptrace.Traces) error
	Take(traceID pcommon.TraceID) (ptrace.Traces, error)
	Delete(traceID pcommon.TraceID) error
	Close() error
}

type pebbleTailStorageExtension struct {
	settings extension.Settings
	cfg      *Config

	storage   tailStorage
	telemetry *metadata.TelemetryBuilder
}

var _ extension.Extension = (*pebbleTailStorageExtension)(nil)

func newExtension(settings extension.Settings, cfg *Config) (*pebbleTailStorageExtension, error) {
	telemetry, err := metadata.NewTelemetryBuilder(settings.TelemetrySettings)
	if err != nil {
		return nil, err
	}
	return &pebbleTailStorageExtension{
		settings:  settings,
		cfg:       cfg,
		telemetry: telemetry,
	}, nil
}

func (e *pebbleTailStorageExtension) Start(ctx context.Context, _ component.Host) error {
	storage, err := newStorage(ctx, e.cfg, e.settings.Logger, e.telemetry)
	if err != nil {
		return err
	}
	e.storage = storage
	return nil
}

func (e *pebbleTailStorageExtension) Shutdown(_ context.Context) error {
	e.telemetry.Shutdown()
	if e.storage == nil {
		return nil
	}
	err := e.storage.Close()
	e.storage = nil
	return err
}

func (e *pebbleTailStorageExtension) Append(traceID pcommon.TraceID, td ptrace.Traces) error {
	err := e.storage.Append(traceID, td)
	e.recordOperation(operationAppend, err)
	return err
}

func (e *pebbleTailStorageExtension) Take(traceID pcommon.TraceID) (ptrace.Traces, error) {
	td, err := e.storage.Take(traceID)
	e.recordOperation(operationTake, err)
	return td, err
}

func (e *pebbleTailStorageExtension) Delete(traceID pcommon.TraceID) error {
	err := e.storage.Delete(traceID)
	e.recordOperation(operationDelete, err)
	return err
}

func (e *pebbleTailStorageExtension) recordOperation(operation string, err error) {
	outcome := outcomeSuccess
	if err != nil {
		outcome = outcomeFailure
	}
	e.telemetry.ExtensionPebbleTailStorageOperations.Add(
		context.Background(),
		1,
		metric.WithAttributes(
			attribute.String(attrOperation, operation),
			attribute.String(attrOutcome, outcome),
		),
	)
}
