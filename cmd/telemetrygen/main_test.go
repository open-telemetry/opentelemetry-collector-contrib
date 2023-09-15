// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/internal/logs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/internal/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/internal/traces"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
)

func TestGenerateTraces(t *testing.T) {
	f := otlpreceiver.NewFactory()
	sink := &consumertest.TracesSink{}
	rCfg := f.CreateDefaultConfig()
	endpoint := testutil.GetAvailableLocalAddress(t)
	rCfg.(*otlpreceiver.Config).GRPC.NetAddr.Endpoint = endpoint
	r, err := f.CreateTracesReceiver(context.Background(), receivertest.NewNopCreateSettings(), rCfg, sink)
	require.NoError(t, err)
	err = r.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, r.Shutdown(context.Background()))
	}()
	cfg := &traces.Config{
		Config: common.Config{
			WorkerCount:        10,
			Rate:               10,
			TotalDuration:      10 * time.Second,
			ReportingInterval:  10,
			Endpoint:           endpoint,
			Insecure:           true,
			UseHTTP:            false,
			Headers:            nil,
			ResourceAttributes: nil,
		},
		NumTraces:   6000,
		ServiceName: "foo",
		StatusCode:  "0",
		LoadSize:    0,
		Batch:       true,
	}
	go func() {
		err = traces.Start(cfg)
		require.NoError(t, err)
	}()
	require.Eventually(t, func() bool {
		return len(sink.AllTraces()) > 0
	}, 10*time.Second, 100*time.Millisecond)
}

func TestGenerateMetrics(t *testing.T) {
	f := otlpreceiver.NewFactory()
	sink := &consumertest.MetricsSink{}
	rCfg := f.CreateDefaultConfig()
	endpoint := testutil.GetAvailableLocalAddress(t)
	rCfg.(*otlpreceiver.Config).GRPC.NetAddr.Endpoint = endpoint
	r, err := f.CreateMetricsReceiver(context.Background(), receivertest.NewNopCreateSettings(), rCfg, sink)
	require.NoError(t, err)
	err = r.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, r.Shutdown(context.Background()))
	}()
	cfg := &metrics.Config{
		Config: common.Config{
			WorkerCount:        10,
			Rate:               10,
			TotalDuration:      10 * time.Second,
			ReportingInterval:  10,
			Endpoint:           endpoint,
			Insecure:           true,
			UseHTTP:            false,
			Headers:            nil,
			ResourceAttributes: nil,
		},
		NumMetrics: 6000,
	}
	go func() {
		err = metrics.Start(cfg)
		require.NoError(t, err)
	}()
	require.Eventually(t, func() bool {
		return len(sink.AllMetrics()) > 0
	}, 10*time.Second, 100*time.Millisecond)
}

func TestGenerateLogs(t *testing.T) {
	f := otlpreceiver.NewFactory()
	sink := &consumertest.LogsSink{}
	rCfg := f.CreateDefaultConfig()
	endpoint := testutil.GetAvailableLocalAddress(t)
	rCfg.(*otlpreceiver.Config).GRPC.NetAddr.Endpoint = endpoint
	r, err := f.CreateLogsReceiver(context.Background(), receivertest.NewNopCreateSettings(), rCfg, sink)
	require.NoError(t, err)
	err = r.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, r.Shutdown(context.Background()))
	}()
	cfg := &logs.Config{
		Config: common.Config{
			WorkerCount:        10,
			Rate:               10,
			TotalDuration:      10 * time.Second,
			ReportingInterval:  10,
			Endpoint:           endpoint,
			Insecure:           true,
			UseHTTP:            false,
			Headers:            nil,
			ResourceAttributes: nil,
		},
		NumLogs: 6000,
	}
	go func() {
		err = logs.Start(cfg)
		require.NoError(t, err)
	}()
	require.Eventually(t, func() bool {
		return len(sink.AllLogs()) > 0
	}, 10*time.Second, 100*time.Millisecond)
}
