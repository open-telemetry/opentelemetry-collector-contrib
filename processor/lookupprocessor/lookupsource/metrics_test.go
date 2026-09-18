// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lookupsource

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/otel/metric/noop"
)

func TestNewReloadMetrics(t *testing.T) {
	ts := component.TelemetrySettings{
		MeterProvider: noop.NewMeterProvider(),
	}

	rm, err := NewReloadMetrics(ts, "test_scope")
	require.NoError(t, err)
	require.NotNil(t, rm)
	assert.NotNil(t, rm.reloads)
	assert.NotNil(t, rm.failures)
}

func TestReloadMetrics_Record(t *testing.T) {
	ts := component.TelemetrySettings{
		MeterProvider: noop.NewMeterProvider(),
	}

	rm, err := NewReloadMetrics(ts, "test_scope")
	require.NoError(t, err)

	ctx := t.Context()

	t.Run("record success", func(t *testing.T) {
		assert.NotPanics(t, func() {
			rm.Record(ctx, true)
		})
	})

	t.Run("record failure", func(t *testing.T) {
		assert.NotPanics(t, func() {
			rm.Record(ctx, false)
		})
	})

	t.Run("nil receiver safe call", func(t *testing.T) {
		var nilMetrics *ReloadMetrics
		assert.NotPanics(t, func() {
			nilMetrics.Record(ctx, true)
			nilMetrics.Record(ctx, false)
		})
	})
}
