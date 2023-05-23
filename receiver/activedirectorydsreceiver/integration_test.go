// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package activedirectorydsreceiver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

/*
TestIntegration test scraping metrics from a running Active Directory domain controller.
The domain controller must be set up locally outside of this test in order for it to pass.
*/
func TestIntegration(t *testing.T) {
	t.Skip("See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/22480")
	t.Parallel()

	fact := NewFactory()

	consumer := &consumertest.MetricsSink{}
	recv, err := fact.CreateMetricsReceiver(context.Background(), receivertest.NewNopCreateSettings(), fact.CreateDefaultConfig(), consumer)

	require.NoError(t, err)

	err = recv.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return len(consumer.AllMetrics()) > 0
	}, 2*time.Minute, 1*time.Second, "failed to receive any metrics")

	actualMetrics := consumer.AllMetrics()[0]
	expectedMetrics, err := golden.ReadMetrics(goldenScrapePath)
	require.NoError(t, err)

	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics, pmetrictest.IgnoreMetricValues(),
		pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))

	err = recv.Shutdown(context.Background())
	require.NoError(t, err)

}
