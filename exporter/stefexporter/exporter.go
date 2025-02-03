// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package stefexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/stefexporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type stefExporter struct{}

func newStefExporter(_ *zap.Logger, _ *Config) *stefExporter {
	return &stefExporter{}
}

func (s *stefExporter) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (s *stefExporter) Shutdown(_ context.Context) error {
	return nil
}

func (s *stefExporter) pushMetrics(_ context.Context, _ pmetric.Metrics) error {
	return nil
}
