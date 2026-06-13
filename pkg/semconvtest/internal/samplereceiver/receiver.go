// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package samplereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/semconvtest/internal/samplereceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
)

var componentType = component.MustNewType("sample_http")

// NewFactory creates a factory for the sample HTTP receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		componentType,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, component.StabilityLevelDevelopment),
	)
}

// Config is an empty configuration for the sample receiver.
type Config struct{}

func createDefaultConfig() component.Config {
	return &Config{}
}

func createMetricsReceiver(
	_ context.Context,
	_ receiver.Settings,
	_ component.Config,
	next consumer.Metrics,
) (receiver.Metrics, error) {
	return &sampleReceiver{next: next}, nil
}

type sampleReceiver struct {
	next consumer.Metrics
}

// Start emits a batch of valid HTTP semconv metrics to the next consumer.
func (r *sampleReceiver) Start(ctx context.Context, _ component.Host) error {
	return r.next.ConsumeMetrics(ctx, generateHTTPServerMetrics())
}

func (*sampleReceiver) Shutdown(_ context.Context) error {
	return nil
}

// generateHTTPServerMetrics builds pmetric.Metrics containing a valid
// http.server.request.duration histogram following HTTP semantic conventions.
func generateHTTPServerMetrics() pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()

	// Valid semconv resource attributes.
	rm.Resource().Attributes().PutStr("service.name", "sample-http-server")
	rm.Resource().Attributes().PutStr("service.version", "1.0.0")

	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("github.com/open-telemetry/opentelemetry-collector-contrib/pkg/semconvtest/internal/samplereceiver")
	sm.Scope().SetVersion("0.1.0")

	m := sm.Metrics().AppendEmpty()
	m.SetName("http.server.request.duration")
	m.SetUnit("s")

	h := m.SetEmptyHistogram()
	h.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

	now := time.Now()
	dp := h.DataPoints().AppendEmpty()
	dp.SetStartTimestamp(pcommon.NewTimestampFromTime(now.Add(-time.Minute)))
	dp.SetTimestamp(pcommon.NewTimestampFromTime(now))
	dp.SetCount(10)
	dp.SetSum(0.5)
	dp.SetMin(0.01)
	dp.SetMax(0.15)
	dp.ExplicitBounds().FromRaw([]float64{
		0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10,
	})
	dp.BucketCounts().FromRaw([]uint64{
		0, 1, 2, 3, 2, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0,
	})

	// Valid semconv data point attributes.
	dp.Attributes().PutStr("http.request.method", "GET")
	dp.Attributes().PutInt("http.response.status_code", 200)
	dp.Attributes().PutStr("http.route", "/api/users")
	dp.Attributes().PutStr("url.scheme", "https")
	dp.Attributes().PutStr("network.protocol.version", "1.1")

	return md
}
