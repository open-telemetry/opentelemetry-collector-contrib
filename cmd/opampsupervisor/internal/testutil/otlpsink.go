// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/internal/testutil"

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

// OTLPSink buffers everything an OTLP/HTTP endpoint receives so tests can assert on it.
type OTLPSink struct {
	Traces  *consumertest.TracesSink
	Metrics *consumertest.MetricsSink
	Logs    *consumertest.LogsSink

	shutdown func()
}

// Shutdown stops the receiver, so that a test asserting on an exact item count cannot
// race against a later batch. Calling it more than once is a no-op.
func (s *OTLPSink) Shutdown() {
	s.shutdown()
}

// StartOTLPSink starts an OTLP/HTTP receiver on endpoint and shuts it down when tb finishes.
func StartOTLPSink(tb testing.TB, endpoint string) *OTLPSink {
	tb.Helper()

	sink := &OTLPSink{
		Traces:  new(consumertest.TracesSink),
		Metrics: new(consumertest.MetricsSink),
		Logs:    new(consumertest.LogsSink),
	}

	factory := otlpreceiver.NewFactory()
	cfg := factory.CreateDefaultConfig().(*otlpreceiver.Config)
	require.NoError(tb, confmap.New().Unmarshal(&cfg.Protocols.HTTP))
	cfg.Protocols.HTTP.Get().ServerConfig.NetAddr.Endpoint = endpoint

	set := receivertest.NewNopSettings(factory.Type())
	_, err := factory.CreateTraces(tb.Context(), set, cfg, sink.Traces)
	require.NoError(tb, err)
	_, err = factory.CreateMetrics(tb.Context(), set, cfg, sink.Metrics)
	require.NoError(tb, err)
	// The factory shares one receiver across signals, so starting and shutting down
	// the logs receiver covers the traces and metrics receivers too.
	r, err := factory.CreateLogs(tb.Context(), set, cfg, sink.Logs)
	require.NoError(tb, err)

	require.NoError(tb, r.Start(tb.Context(), componenttest.NewNopHost()))

	// tb.Context() is canceled before cleanup functions run, so drop the cancellation
	// to give the receiver a live context to shut down with.
	shutdownCtx := context.WithoutCancel(tb.Context())
	var once sync.Once
	sink.shutdown = func() {
		once.Do(func() {
			require.NoError(tb, r.Shutdown(shutdownCtx))
		})
	}
	tb.Cleanup(sink.shutdown)

	return sink
}
