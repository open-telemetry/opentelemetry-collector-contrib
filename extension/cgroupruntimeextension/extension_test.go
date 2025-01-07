// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cgroupruntimeextension

import (
	"context"
	"testing"

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
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			allCalls := 0
			var _err error
			setterMock := func() (undoFunc, error) {
				allCalls++
				return func() { allCalls++ }, _err
			}
			settings := extensiontest.NewNopSettings()
			cg := newCgroupRuntime(test.config, settings.Logger, setterMock, func(_ float64) (undoFunc, error) { return setterMock() })
			ctx := context.Background()

			err := cg.Start(ctx, componenttest.NewNopHost())
			require.NoError(t, err)

			require.NoError(t, cg.Shutdown(ctx))
			require.Equal(t, test.expectedCalls, allCalls)
		})
	}
}
