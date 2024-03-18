// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package apachedruidreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
)

func TestWriteLineProtocol_v2API(t *testing.T) {
	addr := testutil.GetAvailableLocalAddress(t)
	config := &Config{
		HTTPServerSettings: confighttp.HTTPServerSettings{
			Endpoint: addr,
		},
	}
	nextConsumer := new(mockConsumer)

	receiver, outerErr := NewFactory().CreateMetricsReceiver(context.Background(), receivertest.NewNopCreateSettings(), config, nextConsumer)
	require.NoError(t, outerErr)
	require.NotNil(t, receiver)

	require.NoError(t, receiver.Start(context.Background(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, receiver.Shutdown(context.Background())) })

	t.Run("Apache-Druid-HTTP-emitter-client", func(t *testing.T) {
		nextConsumer.lastMetricsConsumed = pmetric.NewMetrics()

	})

}

type mockConsumer struct {
	lastMetricsConsumed pmetric.Metrics
}

func (m *mockConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (m *mockConsumer) ConsumeMetrics(_ context.Context, md pmetric.Metrics) error {
	m.lastMetricsConsumed = pmetric.NewMetrics()
	md.CopyTo(m.lastMetricsConsumed)
	return nil
}
