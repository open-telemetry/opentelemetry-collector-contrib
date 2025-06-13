// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package stefreceiver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/stefexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/stefreceiver/internal/metadata"
)

func TestRoundtrip(t *testing.T) {
	sink := &consumertest.MetricsSink{}
	settings := receivertest.NewNopSettings(metadata.Type)
	settings.Logger, _ = zap.NewDevelopment()
	m, err := NewFactory().CreateMetrics(context.Background(), settings, createDefaultConfig(), sink)
	t.Cleanup(func() {
		err = m.Shutdown(context.Background())
		require.NoError(t, err)
	})
	require.NoError(t, m.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, err)
	cfg := stefexporter.NewFactory().CreateDefaultConfig().(*stefexporter.Config)
	cfg.Endpoint = "localhost:4320"
	cfg.TLSSetting.Insecure = true
	exporterSettings := exportertest.NewNopSettings(metadata.Type)
	exporterSettings.Logger, _ = zap.NewDevelopment()
	exporter, err := stefexporter.NewFactory().CreateMetrics(context.Background(), exporterSettings, cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		err = exporter.Shutdown(context.Background())
		require.NoError(t, err)
	})

	require.NoError(t, exporter.Start(context.Background(), componenttest.NewNopHost()))
	data := pmetric.NewMetrics()
	metricPoint := data.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	metricPoint.SetName("foo")
	gauge := metricPoint.SetEmptyGauge()
	dp := gauge.DataPoints().AppendEmpty()
	dp.SetIntValue(1)
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	err = exporter.ConsumeMetrics(context.Background(), data)
	require.NoError(t, err)
	assert.EventuallyWithT(t, func(tt *assert.CollectT) {
		assert.Len(tt, sink.AllMetrics(), 1)
	}, 1*time.Minute, 10*time.Millisecond)
}
