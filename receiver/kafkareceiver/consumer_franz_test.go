// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver"

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"
)

func setFranzGo(tb testing.TB, value bool) {
	currentFranzState := franzGoConsumerFeatureGate.IsEnabled()
	require.NoError(tb, featuregate.GlobalRegistry().Set(franzGoConsumerFeatureGate.ID(), value))
	tb.Cleanup(func() {
		require.NoError(tb, featuregate.GlobalRegistry().Set(franzGoConsumerFeatureGate.ID(), currentFranzState))
	})
}
