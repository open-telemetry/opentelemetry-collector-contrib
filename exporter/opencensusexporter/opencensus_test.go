// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opencensusexporter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/testdata"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opencensusexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/opencensusreceiver"
)

func TestSendTraces(t *testing.T) {
	sink := new(consumertest.TracesSink)
	rFactory := opencensusreceiver.NewFactory()
	rCfg := rFactory.CreateDefaultConfig().(*opencensusreceiver.Config)
	endpoint := testutil.GetAvailableLocalAddress(t)
	rCfg.NetAddr.Endpoint = endpoint
	set := receivertest.NewNopSettings(metadata.Type)
	recv, err := rFactory.CreateTraces(t.Context(), set, rCfg, sink)
	assert.NoError(t, err)
	assert.NoError(t, recv.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() {
		assert.NoError(t, recv.Shutdown(t.Context()))
	})

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.ClientConfig = configgrpc.ClientConfig{
		Endpoint: endpoint,
		TLS: configtls.ClientConfig{
			Insecure: true,
		},
	}
	cfg.NumWorkers = 1
	exp, err := factory.CreateTraces(t.Context(), exportertest.NewNopSettings(metadata.Type), cfg)
	require.NoError(t, err)
	require.NotNil(t, exp)
	host := componenttest.NewNopHost()
	require.NoError(t, exp.Start(t.Context(), host))
	t.Cleanup(func() {
		assert.NoError(t, exp.Shutdown(t.Context()))
	})

	td := testdata.GenerateTraces(1)
	assert.NoError(t, exp.ConsumeTraces(t.Context(), td))
	assert.Eventually(t, func() bool {
		return len(sink.AllTraces()) == 1
	}, 10*time.Second, 5*time.Millisecond)
	traces := sink.AllTraces()
	require.Len(t, traces, 1)
	assert.Equal(t, td, traces[0])

	sink.Reset()
	// Sending data no Node.
	td.ResourceSpans().At(0).Resource().Attributes().Clear()
	newData := ptrace.NewTraces()
	td.CopyTo(newData)
	assert.NoError(t, exp.ConsumeTraces(t.Context(), newData))
	assert.Eventually(t, func() bool {
		return len(sink.AllTraces()) == 1
	}, 10*time.Second, 5*time.Millisecond)
	traces = sink.AllTraces()
	require.Len(t, traces, 1)
	assert.Equal(t, newData, traces[0])
}

func TestSendTraces_NoBackend(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.ClientConfig = configgrpc.ClientConfig{
		Endpoint: "localhost:56569",
		TLS: configtls.ClientConfig{
			Insecure: true,
		},
	}
	exp, err := factory.CreateTraces(t.Context(), exportertest.NewNopSettings(metadata.Type), cfg)
	require.NoError(t, err)
	require.NotNil(t, exp)
	host := componenttest.NewNopHost()
	require.NoError(t, exp.Start(t.Context(), host))
	t.Cleanup(func() {
		assert.NoError(t, exp.Shutdown(t.Context()))
	})

	td := testdata.GenerateTraces(1)
	for i := 0; i < 10000; i++ {
		assert.Error(t, exp.ConsumeTraces(t.Context(), td))
	}
}

func TestSendTraces_AfterStop(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.ClientConfig = configgrpc.ClientConfig{
		Endpoint: "localhost:56569",
		TLS: configtls.ClientConfig{
			Insecure: true,
		},
	}
	exp, err := factory.CreateTraces(t.Context(), exportertest.NewNopSettings(metadata.Type), cfg)
	require.NoError(t, err)
	require.NotNil(t, exp)
	host := componenttest.NewNopHost()
	require.NoError(t, exp.Start(t.Context(), host))
	assert.NoError(t, exp.Shutdown(t.Context()))

	td := testdata.GenerateTraces(1)
	assert.Error(t, exp.ConsumeTraces(t.Context(), td))
}

func TestSendMetrics(t *testing.T) {
	sink := new(consumertest.MetricsSink)
	rFactory := opencensusreceiver.NewFactory()
	rCfg := rFactory.CreateDefaultConfig().(*opencensusreceiver.Config)
	endpoint := testutil.GetAvailableLocalAddress(t)
	rCfg.NetAddr.Endpoint = endpoint
	set := receivertest.NewNopSettings(metadata.Type)
	recv, err := rFactory.CreateMetrics(t.Context(), set, rCfg, sink)
	assert.NoError(t, err)
	assert.NoError(t, recv.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() {
		assert.NoError(t, recv.Shutdown(t.Context()))
	})

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.ClientConfig = configgrpc.ClientConfig{
		Endpoint: endpoint,
		TLS: configtls.ClientConfig{
			Insecure: true,
		},
	}
	cfg.NumWorkers = 1
	exp, err := factory.CreateMetrics(t.Context(), exportertest.NewNopSettings(metadata.Type), cfg)
	require.NoError(t, err)
	require.NotNil(t, exp)
	host := componenttest.NewNopHost()
	require.NoError(t, exp.Start(t.Context(), host))
	t.Cleanup(func() {
		assert.NoError(t, exp.Shutdown(t.Context()))
	})

	md := testdata.GenerateMetrics(1)
	assert.NoError(t, exp.ConsumeMetrics(t.Context(), md))
	assert.Eventually(t, func() bool {
		return len(sink.AllMetrics()) == 1
	}, 10*time.Second, 5*time.Millisecond)
	metrics := sink.AllMetrics()
	require.Len(t, metrics, 1)
	assert.Equal(t, md, metrics[0])

	// Sending data no node.
	sink.Reset()
	md.ResourceMetrics().At(0).Resource().Attributes().Clear()
	assert.NoError(t, exp.ConsumeMetrics(t.Context(), md))
	assert.Eventually(t, func() bool {
		return len(sink.AllMetrics()) == 1
	}, 10*time.Second, 5*time.Millisecond)
	metrics = sink.AllMetrics()
	require.Len(t, metrics, 1)
	assert.Equal(t, md, metrics[0])
}

func TestSendMetrics_NoBackend(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.ClientConfig = configgrpc.ClientConfig{
		Endpoint: "localhost:56569",
		TLS: configtls.ClientConfig{
			Insecure: true,
		},
	}
	exp, err := factory.CreateMetrics(t.Context(), exportertest.NewNopSettings(metadata.Type), cfg)
	require.NoError(t, err)
	require.NotNil(t, exp)
	host := componenttest.NewNopHost()
	require.NoError(t, exp.Start(t.Context(), host))
	t.Cleanup(func() {
		assert.NoError(t, exp.Shutdown(t.Context()))
	})

	md := testdata.GenerateMetrics(1)
	for i := 0; i < 10000; i++ {
		assert.Error(t, exp.ConsumeMetrics(t.Context(), md))
	}
}

func TestSendMetrics_AfterStop(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.ClientConfig = configgrpc.ClientConfig{
		Endpoint: "localhost:56569",
		TLS: configtls.ClientConfig{
			Insecure: true,
		},
	}
	exp, err := factory.CreateMetrics(t.Context(), exportertest.NewNopSettings(metadata.Type), cfg)
	require.NoError(t, err)
	require.NotNil(t, exp)
	host := componenttest.NewNopHost()
	require.NoError(t, exp.Start(t.Context(), host))
	assert.NoError(t, exp.Shutdown(t.Context()))

	md := testdata.GenerateMetrics(1)
	assert.Error(t, exp.ConsumeMetrics(t.Context(), md))
}
