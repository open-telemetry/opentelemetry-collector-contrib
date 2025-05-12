// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package offset

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdjuster(t *testing.T) {
	metrics := func(start, ts pcommon.Timestamp) pmetric.Metrics {
		md := pmetric.NewMetrics()
		ms := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()

		sum := ms.AppendEmpty().SetEmptySum().DataPoints().AppendEmpty()
		sum.SetStartTimestamp(start)
		sum.SetTimestamp(ts)

		gauge := ms.AppendEmpty().SetEmptyGauge().DataPoints().AppendEmpty()
		gauge.SetStartTimestamp(start)
		gauge.SetTimestamp(ts)

		histogram := ms.AppendEmpty().SetEmptyHistogram().DataPoints().AppendEmpty()
		histogram.SetStartTimestamp(start)
		histogram.SetTimestamp(ts)

		exp := ms.AppendEmpty().SetEmptyExponentialHistogram().DataPoints().AppendEmpty()
		exp.SetStartTimestamp(start)
		exp.SetTimestamp(ts)

		summary := ms.AppendEmpty().SetEmptySummary().DataPoints().AppendEmpty()
		summary.SetStartTimestamp(start)
		summary.SetTimestamp(ts)
		return md
	}

	now := time.Now()
	start := pcommon.NewTimestampFromTime(now.Add(-time.Minute))
	ts := pcommon.NewTimestampFromTime(now)

	adj := NewAdjuster(componenttest.NewNopTelemetrySettings(), time.Minute)
	got, err := adj.AdjustMetrics(context.TODO(), metrics(0, ts))
	require.NoError(t, err)

	want := metrics(start, ts)
	assert.Equal(t, want.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics(), got.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics())
}
