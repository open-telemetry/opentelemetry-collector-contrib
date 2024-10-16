// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transformer // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/transformer"

import (
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/migrate"
)

// SpanEventSignalNameChange is an transformer that powers the [Span Event's rename_events] change.
// [Span Event's rename_events]: https://opentelemetry.io/docs/specs/otel/schemas/file_format_v1.1.0/#rename_events-transformation
type SpanEventSignalNameChange struct {
	SignalNameChange migrate.SignalNameChange
}

func (c SpanEventSignalNameChange) IsMigrator() {}

func (c SpanEventSignalNameChange) Do(ss migrate.StateSelector, span ptrace.Span) error {
	for e := 0; e < span.Events().Len(); e++ {
		event := span.Events().At(e)
		c.SignalNameChange.Do(ss, event)
	}
	return nil
}

// MetricSignalNameChange is an transformer that powers the [Metric's rename_metrics] change.
// [Metric's rename_metrics]: https://opentelemetry.io/docs/specs/otel/schemas/file_format_v1.1.0/#rename_metrics-transformation
type MetricSignalNameChange struct {
	SignalNameChange migrate.SignalNameChange
}

func (c MetricSignalNameChange) IsMigrator() {}

func (c MetricSignalNameChange) Do(ss migrate.StateSelector, metric pmetric.Metric) error {
	c.SignalNameChange.Do(ss, metric)
	return nil
}
