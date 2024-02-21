// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelarrowexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/otelarrowexporter"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"google.golang.org/grpc"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/otelarrowexporter/internal/arrow"
)

// baseExporter is used as the basis for all OpenTelemetry signal types.
type baseExporter struct {
	// config is the active component.Config.
	config *Config

	// settings are the active collector-wide settings.
	settings exporter.CreateSettings

	// TODO: implementation
}

type streamClientFactory func(conn *grpc.ClientConn) arrow.StreamClientFunc

// newExporter configures a new exporter using the associated stream factory for Arrow.
func newExporter(cfg component.Config, set exporter.CreateSettings, _ streamClientFactory) (*baseExporter, error) {
	// TODO: Implementation.
	oCfg, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("unrecognized configuration type: %T", cfg)
	}
	if oCfg.Endpoint == "" {
		return nil, errors.New("OTel-Arrow exporter config requires an Endpoint")
	}
	return &baseExporter{
		config:   oCfg,
		settings: set,
	}, nil
}

// start configures and starts the gRPC client connection.
func (e *baseExporter) start(ctx context.Context, host component.Host) (err error) {
	// TODO: Implementation: the following is a placeholder used
	// to satisfy gRPC configuration-related configuration errors.
	if _, err = e.config.ClientConfig.ToClientConn(ctx, host, e.settings.TelemetrySettings); err != nil {
		return err
	}
	return nil
}

func (e *baseExporter) shutdown(_ context.Context) error {
	// TODO: Implementation.
	return nil
}

func (e *baseExporter) pushTraces(_ context.Context, _ ptrace.Traces) error {
	// TODO: Implementation.
	return nil
}

func (e *baseExporter) pushMetrics(_ context.Context, _ pmetric.Metrics) error {
	// TODO: Implementation.
	return nil
}

func (e *baseExporter) pushLogs(_ context.Context, _ plog.Logs) error {
	// TODO: Implementation.
	return nil
}
