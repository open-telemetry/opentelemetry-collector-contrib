// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelarrowexporter

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/otelarrowexporter/internal/arrow"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type baseExporter struct {
	// config is the active component.Config.
	config *Config

	// settings are the active collector-wide settings.
	settings exporter.CreateSettings

	// TODO: implementation
}

type streamClientFactory func(cfg *Config, conn *grpc.ClientConn) arrow.StreamClientFunc

// newExporter configures a new exporter using the associated stream factory for Arrow.
func newExporter(cfg component.Config, set exporter.CreateSettings, streamClientFactory streamClientFactory) (*baseExporter, error) {
	// TODO: Implementation.
	oCfg, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("unrecognized configuration type: %T", cfg)
	}
	return &baseExporter{
		config:   oCfg,
		settings: set,
	}, nil
}

// start configures and starts the gRPC client connection.
func (e *baseExporter) start(ctx context.Context, host component.Host) (err error) {
	// TODO: Implementation.
	return nil
}

func (e *baseExporter) shutdown(ctx context.Context) error {
	// TODO: Implementation.
	return nil
}

func (e *baseExporter) pushTraces(ctx context.Context, td ptrace.Traces) error {
	// TODO: Implementation.
	return nil
}

func (e *baseExporter) pushMetrics(ctx context.Context, md pmetric.Metrics) error {
	// TODO: Implementation.
	return nil
}

func (e *baseExporter) pushLogs(ctx context.Context, ld plog.Logs) error {
	// TODO: Implementation.
	return nil
}
