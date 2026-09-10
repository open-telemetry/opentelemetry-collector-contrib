// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lookupsource // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor/lookupsource"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/otel/metric"
)

// ReloadMetrics counts file-based source reloads. A nil value is safe to use.
type ReloadMetrics struct {
	reloads  metric.Int64Counter
	failures metric.Int64Counter
}

// NewReloadMetrics creates the reload counters on the meter for scopeName.
func NewReloadMetrics(ts component.TelemetrySettings, scopeName string) (*ReloadMetrics, error) {
	meter := ts.MeterProvider.Meter(scopeName)

	reloads, err := meter.Int64Counter(
		"lookup_source_reloads",
		metric.WithDescription("Number of successful reloads of a file-based lookup source."),
		metric.WithUnit("{reload}"),
	)
	if err != nil {
		return nil, err
	}

	failures, err := meter.Int64Counter(
		"lookup_source_reload_failures",
		metric.WithDescription("Number of failed reloads of a file-based lookup source."),
		metric.WithUnit("{reload}"),
	)
	if err != nil {
		return nil, err
	}

	return &ReloadMetrics{reloads: reloads, failures: failures}, nil
}

// Record adds to the success or failure counter. It works as a ReloadCallback.
func (m *ReloadMetrics) Record(ctx context.Context, success bool) {
	if m == nil {
		return
	}
	if success {
		m.reloads.Add(ctx, 1)
	} else {
		m.failures.Add(ctx, 1)
	}
}
