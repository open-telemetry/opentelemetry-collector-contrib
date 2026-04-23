// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cgroupruntimeextension

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension/extensiontest"
)

func TestExtension(t *testing.T) {
	tests := []struct {
		name          string
		config        *Config
		expectedCalls int
	}{
		{
			name: "all enabled",
			config: &Config{
				GoMaxProcs: GoMaxProcsConfig{
					Enabled: true,
				},
				GoMemLimit: GoMemLimitConfig{
					Enabled: true,
					Ratio:   0.5,
				},
			},
			expectedCalls: 4,
		},
		{
			name: "everything disabled",
			config: &Config{
				GoMaxProcs: GoMaxProcsConfig{
					Enabled: false,
				},
				GoMemLimit: GoMemLimitConfig{
					Enabled: false,
				},
			},
			expectedCalls: 0,
		},
		{
			name: "memory limit with refresh interval",
			config: &Config{
				GoMaxProcs: GoMaxProcsConfig{
					Enabled: false,
				},
				GoMemLimit: GoMemLimitConfig{
					Enabled:         true,
					Ratio:           0.8,
					RefreshInterval: 15 * time.Second,
				},
			},
			expectedCalls: 2,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			allCalls := 0
			var _err error
			maxProcsSetterMock := func() (undoFunc, error) {
				allCalls++
				return func() { allCalls++ }, _err
			}
			memLimitSetterMock := func(_ float64) (undoFunc, error) {
				allCalls++
				return func() { allCalls++ }, _err
			}
			settings := extensiontest.NewNopSettings(extensiontest.NopType)
			cg := newCgroupRuntime(test.config, settings.Logger, maxProcsSetterMock, memLimitSetterMock)
			ctx := t.Context()

			err := cg.Start(ctx, componenttest.NewNopHost())
			require.NoError(t, err)

			require.NoError(t, cg.Shutdown(ctx))
			require.Equal(t, test.expectedCalls, allCalls)
		})
	}
}

func TestMemLimitRefreshStopsOnShutdown(t *testing.T) {
	var memLimitCalls atomic.Int64

	config := &Config{
		GoMaxProcs: GoMaxProcsConfig{Enabled: false},
		GoMemLimit: GoMemLimitConfig{
			Enabled:         true,
			Ratio:           0.8,
			RefreshInterval: 10 * time.Millisecond,
		},
	}

	maxProcsSetterMock := func() (undoFunc, error) {
		return func() {}, nil
	}
	memLimitSetterMock := func(_ float64) (undoFunc, error) {
		memLimitCalls.Add(1)
		return func() {}, nil
	}

	settings := extensiontest.NewNopSettings(extensiontest.NopType)
	cg := newCgroupRuntime(config, settings.Logger, maxProcsSetterMock, memLimitSetterMock)
	ctx := t.Context()

	require.NoError(t, cg.Start(ctx, componenttest.NewNopHost()))
	require.Eventually(t, func() bool {
		// one call happens at startup; second verifies ticker-based refresh is active.
		return memLimitCalls.Load() >= 2
	}, 500*time.Millisecond, 10*time.Millisecond)

	require.NoError(t, cg.Shutdown(ctx))

	callsAfterShutdown := memLimitCalls.Load()
	time.Sleep(50 * time.Millisecond)
	require.Equal(t, callsAfterShutdown, memLimitCalls.Load())
}
