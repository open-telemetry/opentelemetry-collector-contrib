// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package awslambdareceiver

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awslambdareceiver/internal/metadata"
)

func TestCreateLogsReceiver(t *testing.T) {
	cfg := createDefaultConfig()
	r, err := createLogsReceiver(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, r)
}

func TestCreateMetricsReceiver(t *testing.T) {
	cfg := createDefaultConfig()
	r, err := createMetricsReceiver(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, r)
}
