// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package operator // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/operator"

import (
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/migrate"
)

type LogOperator interface {
	migrate.Migrator
	Do(ss migrate.StateSelector, log plog.LogRecord) error
}

type MetricOperator interface {
	migrate.Migrator
	Do(ss migrate.StateSelector, metric pmetric.Metric) error
}

type SpanOperator interface {
	migrate.Migrator
	Do(ss migrate.StateSelector, signal ptrace.Span) error
}
