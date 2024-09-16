// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package operator // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/operator"

import (
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/migrate"
)

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

// MetricSignalNameChange is a similar type as SpanEventSignalNameChange, but for metrics
type MetricSignalNameChange struct {
	SignalNameChange migrate.SignalNameChange
}

func (c MetricSignalNameChange) IsMigrator() {}

func (c MetricSignalNameChange) Do(ss migrate.StateSelector, metric pmetric.Metric) error {
	c.SignalNameChange.Do(ss, metric)
	return nil
}
