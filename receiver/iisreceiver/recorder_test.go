// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package iisreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/iisreceiver"

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/iisreceiver/internal/metadata"
)

func TestBuildTotalPerfCounterRecordersFromConfig(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		metricsConfig := metadata.DefaultMetricsConfig()
		metricsConfig.IisThreadActive.Enabled = false

		recorders := buildTotalPerfCounterRecordersFromConfig(metricsConfig)
		require.Nil(t, recorders)
	})

	t.Run("enabled", func(t *testing.T) {
		metricsConfig := metadata.DefaultMetricsConfig()
		metricsConfig.IisThreadActive.Enabled = true

		recorders := buildTotalPerfCounterRecordersFromConfig(metricsConfig)
		require.Len(t, recorders, 1)
		require.Equal(t, "Process", recorders[0].object)
		require.Equal(t, "_Total", recorders[0].instance)
		require.Equal(t, []string{"Thread Count"}, sortedRecorderNames(recorders[0].recorders))
	})
}

func TestBuildSitePerfCounterRecordersFromConfig(t *testing.T) {
	t.Run("all disabled", func(t *testing.T) {
		metricsConfig := metadata.DefaultMetricsConfig()
		metricsConfig.IisConnectionActive.Enabled = false
		metricsConfig.IisNetworkIo.Enabled = false
		metricsConfig.IisConnectionAttemptCount.Enabled = false
		metricsConfig.IisRequestCount.Enabled = false
		metricsConfig.IisNetworkFileCount.Enabled = false
		metricsConfig.IisConnectionAnonymous.Enabled = false
		metricsConfig.IisNetworkBlocked.Enabled = false
		metricsConfig.IisUptime.Enabled = false

		recorders := buildSitePerfCounterRecordersFromConfig(metricsConfig)
		require.Nil(t, recorders)
	})

	t.Run("partial enabled", func(t *testing.T) {
		metricsConfig := metadata.DefaultMetricsConfig()
		metricsConfig.IisConnectionActive.Enabled = false
		metricsConfig.IisNetworkIo.Enabled = true
		metricsConfig.IisConnectionAttemptCount.Enabled = false
		metricsConfig.IisRequestCount.Enabled = false
		metricsConfig.IisNetworkFileCount.Enabled = true
		metricsConfig.IisConnectionAnonymous.Enabled = false
		metricsConfig.IisNetworkBlocked.Enabled = false
		metricsConfig.IisUptime.Enabled = false

		recorders := buildSitePerfCounterRecordersFromConfig(metricsConfig)
		require.Len(t, recorders, 1)
		require.Equal(t, "Web Service", recorders[0].object)
		require.Equal(t, "*", recorders[0].instance)
		require.Equal(
			t,
			[]string{"Total Bytes Received", "Total Bytes Sent", "Total Files Received", "Total Files Sent"},
			sortedRecorderNames(recorders[0].recorders),
		)
	})
}

func TestBuildAppPoolPerfCounterRecordersFromConfig(t *testing.T) {
	t.Run("all disabled", func(t *testing.T) {
		metricsConfig := metadata.DefaultMetricsConfig()
		metricsConfig.IisRequestRejected.Enabled = false
		metricsConfig.IisRequestQueueCount.Enabled = false
		metricsConfig.IisApplicationPoolState.Enabled = false
		metricsConfig.IisApplicationPoolUptime.Enabled = false

		recorders := buildAppPoolPerfCounterRecordersFromConfig(metricsConfig)
		require.Empty(t, recorders)
	})

	t.Run("partial enabled", func(t *testing.T) {
		metricsConfig := metadata.DefaultMetricsConfig()
		metricsConfig.IisRequestRejected.Enabled = false
		metricsConfig.IisRequestQueueCount.Enabled = true
		metricsConfig.IisApplicationPoolState.Enabled = true
		metricsConfig.IisApplicationPoolUptime.Enabled = false

		recorders := buildAppPoolPerfCounterRecordersFromConfig(metricsConfig)
		require.Len(t, recorders, 2)

		require.Equal(t, "HTTP Service Request Queues", recorders[0].object)
		require.Equal(t, "*", recorders[0].instance)
		require.Equal(t, []string{"CurrentQueueSize"}, sortedRecorderNames(recorders[0].recorders))

		require.Equal(t, "APP_POOL_WAS", recorders[1].object)
		require.Equal(t, "*", recorders[1].instance)
		require.Equal(t, []string{"Current Application Pool State"}, sortedRecorderNames(recorders[1].recorders))
	})
}

func sortedRecorderNames(recorders map[string]recordFunc) []string {
	names := make([]string, 0, len(recorders))
	for name := range recorders {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
