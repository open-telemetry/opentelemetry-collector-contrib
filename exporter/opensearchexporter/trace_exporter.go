// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package opensearchexporter contains an opentelemetry-collector exporter
// for OpenSearch.
package opensearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type tracesExporter interface {
	Shutdown(ctx context.Context) error
	pushTraceData(
		ctx context.Context,
		td ptrace.Traces,
	) error
}

func newTracesExporter(logger *zap.Logger, cfg *Config) (tracesExporter, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return newSSOTracesExporter(logger, cfg)

}
