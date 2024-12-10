// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vcenterreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestCreateMetrics(t *testing.T) {
	testCases := []struct {
		desc   string
		testFn func(t *testing.T)
	}{
		{
			desc: "Default config",
			testFn: func(t *testing.T) {
				t.Parallel()
				_, err := createMetricsReceiver(
					context.Background(),
					receivertest.NewNopSettings(),
					createDefaultConfig(),
					consumertest.NewNop(),
				)
				require.NoError(t, err)
			},
		},
		{
			desc: "Nil config",
			testFn: func(t *testing.T) {
				t.Parallel()
				_, err := createMetricsReceiver(
					context.Background(),
					receivertest.NewNopSettings(),
					nil,
					consumertest.NewNop(),
				)
				require.ErrorIs(t, err, errConfigNotVcenter)
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.desc, testCase.testFn)
	}
}
