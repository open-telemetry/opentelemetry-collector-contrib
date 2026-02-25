// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadata

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

type testDataSet int

const (
	testDataSetDefault testDataSet = iota
	testDataSetAll
	testDataSetNone
)

func TestMetricsBuilder(t *testing.T) {
	tests := []struct {
		name        string
		resAttrsSet testDataSet
	}{
		{
			name: "default",
		},
		{
			name:        "all_set",
			resAttrsSet: testDataSetAll,
		},
		{
			name:        "none_set",
			resAttrsSet: testDataSetNone,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := pcommon.Timestamp(1_000_000_000)
			ts := pcommon.Timestamp(1_000_001_000)
			observedZapCore, observedLogs := observer.New(zap.WarnLevel)
			settings := receivertest.NewNopSettings(Type)
			settings.Logger = zap.New(observedZapCore)
			mb := NewMetricsBuilder(loadMetricsBuilderConfig(t, tt.name), settings, WithStartTime(start))

			expectedWarnings := 0

			assert.Equal(t, expectedWarnings, observedLogs.Len())

			var val1 float64 = 1
			attributes := map[string]*string{}
			mb.AddDataPoint("resId1", "metric1", "count", "unit", attributes, ts, val1)

			rb := mb.NewResourceBuilder()
			rb.SetAzuremonitorTenantID("tenant-id-val")
			rb.SetAzuremonitorSubscriptionID("subscription-id-val")
			res := rb.Emit()
			metrics := mb.Emit(WithResource(res))
			assert.Equal(t, 1, metrics.ResourceMetrics().Len())
			rm := metrics.ResourceMetrics().At(0)

			if tt.resAttrsSet == testDataSetNone {
				assert.Equal(t, 0, rm.Resource().Attributes().Len())
			} else {
				attrVal, ok := rm.Resource().Attributes().Get("azuremonitor.subscription_id")
				assert.True(t, ok)
				assert.Equal(t, "subscription-id-val", attrVal.Str())
				attrVal, ok = rm.Resource().Attributes().Get("azuremonitor.tenant_id")
				assert.True(t, ok)
				assert.Equal(t, "tenant-id-val", attrVal.Str())
			}

			assert.Equal(t, 1, rm.ScopeMetrics().Len())
		})
	}
}
