// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package e2etest

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/pkg/traces"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
)

func TestGenerateTraces(t *testing.T) {
	f := otlpreceiver.NewFactory()
	sink := &consumertest.TracesSink{}
	rCfg := f.CreateDefaultConfig()
	endpoint := testutil.GetAvailableLocalAddress(t)
	rCfg.(*otlpreceiver.Config).GRPC.NetAddr.Endpoint = endpoint
	r, err := f.CreateTraces(context.Background(), receivertest.NewNopSettings(), rCfg, sink)
	require.NoError(t, err)
	err = r.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, r.Shutdown(context.Background()))
	}()
	cfg := traces.NewConfig()
	cfg.WorkerCount = 10
	cfg.Rate = 10
	cfg.TotalDuration = 10 * time.Second
	cfg.ReportingInterval = 10
	cfg.CustomEndpoint = endpoint
	cfg.Insecure = true
	cfg.SkipSettingGRPCLogger = true
	cfg.NumTraces = 6000
	go func() {
		err = traces.Start(cfg)
		assert.NoError(t, err)
	}()
	require.Eventually(t, func() bool {
		return len(sink.AllTraces()) > 0
	}, 10*time.Second, 100*time.Millisecond)
}
