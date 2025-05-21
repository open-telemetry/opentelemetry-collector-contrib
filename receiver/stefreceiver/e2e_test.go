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
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/stefexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/stefreceiver/internal/metadata"
)

func genMetrics() pmetric.Metrics {
	data := pmetric.NewMetrics()
	metricPoint := data.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	metricPoint.SetName("foo")
	gauge := metricPoint.SetEmptyGauge()
	dp := gauge.DataPoints().AppendEmpty()
	dp.SetIntValue(1)
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	return data
}

func createReceiver(t *testing.T, endpoint string) (receiver.Metrics, *consumertest.MetricsSink) {
	sink := &consumertest.MetricsSink{}
	settings := receivertest.NewNopSettings(metadata.Type)
	settings.Logger, _ = zap.NewDevelopment()
	rcvCfg := createDefaultConfig()
	rcvCfg.(*Config).NetAddr.Endpoint = endpoint
	m, err := NewFactory().CreateMetrics(context.Background(), settings, rcvCfg, sink)
	require.NoError(t, m.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, err)
	return m, sink
}

func createExporter(t *testing.T, endpoint string) exporter.Metrics {
	cfg := stefexporter.NewFactory().CreateDefaultConfig().(*stefexporter.Config)
	cfg.Endpoint = endpoint
	cfg.TLSSetting.Insecure = true
	settings := exportertest.NewNopSettings(metadata.Type)
	settings.Logger, _ = zap.NewDevelopment()
	exp, err := stefexporter.NewFactory().CreateMetrics(context.Background(), settings, cfg)
	require.NoError(t, err)
	require.NoError(t, exp.Start(context.Background(), componenttest.NewNopHost()))
	return exp
}

func TestRoundtrip(t *testing.T) {
	endpoint := testutil.GetAvailableLocalAddress(t)
	m, sink := createReceiver(t, endpoint)
	t.Cleanup(func() { require.NoError(t, m.Shutdown(context.Background())) })

	exporter := createExporter(t, endpoint)
	t.Cleanup(func() { require.NoError(t, exporter.Shutdown(context.Background())) })

	err := exporter.ConsumeMetrics(context.Background(), genMetrics())
	require.NoError(t, err)
	assert.EventuallyWithT(t, func(tt *assert.CollectT) {
		assert.Len(tt, sink.AllMetrics(), 1)
	}, 1*time.Minute, 10*time.Millisecond)
}

func TestShutdownWhenConnected(t *testing.T) {
	endpoint := testutil.GetAvailableLocalAddress(t)
	receiver, sink := createReceiver(t, endpoint)
	exporter := createExporter(t, endpoint)

	require.NoError(t, exporter.ConsumeMetrics(context.Background(), genMetrics()))

	assert.EventuallyWithT(
		t, func(tt *assert.CollectT) {
			assert.Len(tt, sink.AllMetrics(), 1)
		}, 1*time.Minute, 10*time.Millisecond,
	)

	// Try shutdown receiver before shutting down exporter.
	// This means there is an active connection at the receiver.
	// Previously we had a bug causing the receiver Shutdown to hang forever
	// in this situation.
	require.NoError(t, receiver.Shutdown(context.Background()))

	require.NoError(t, exporter.Shutdown(context.Background()))
}
