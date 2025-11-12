// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tailsamplingprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"
)

func TestNotHelper(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		actual, err := getNewNotPolicy(componenttest.NewNopTelemetrySettings(), &NotCfg{
			SubPolicy: NotSubPolicyCfg{
				sharedPolicyCfg: sharedPolicyCfg{
					Name:       "test-not-policy-1",
					Type:       Latency,
					LatencyCfg: LatencyCfg{ThresholdMs: 100},
				},
			},
		}, nil)
		require.NoError(t, err)

		expected := sampling.NewNot(zap.NewNop(), sampling.NewLatency(componenttest.NewNopTelemetrySettings(), 100, 0))
		assert.Equal(t, expected, actual)
	})

	t.Run("unsupported sampling policy type", func(t *testing.T) {
		_, err := getNewNotPolicy(componenttest.NewNopTelemetrySettings(), &NotCfg{
			SubPolicy: NotSubPolicyCfg{
				sharedPolicyCfg: sharedPolicyCfg{
					Name: "test-not-policy-2",
					Type: Not, // nested not is not allowed
				},
			},
		}, nil)
		require.EqualError(t, err, "unknown sampling policy type not")
	})
}
