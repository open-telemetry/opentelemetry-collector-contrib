// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsfirehosereceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestValidConfig(t *testing.T) {
	err := componenttest.CheckConfigStruct(createDefaultConfig())
	require.NoError(t, err)
}

func TestCreateMetrics(t *testing.T) {
	r, err := createMetricsReceiver(
		context.Background(),
		receivertest.NewNopSettings(),
		createDefaultConfig(),
		consumertest.NewNop(),
	)
	require.NoError(t, err)
	require.NotNil(t, r)
}

func TestCreateLogsReceiver(t *testing.T) {
	r, err := createLogsReceiver(
		context.Background(),
		receivertest.NewNopSettings(),
		createDefaultConfig(),
		consumertest.NewNop(),
	)
	require.NoError(t, err)
	require.NotNil(t, r)
}

func TestValidateRecordType(t *testing.T) {
	require.NoError(t, validateRecordType(defaultMetricsRecordType))
	require.NoError(t, validateRecordType(defaultLogsRecordType))
	require.Error(t, validateRecordType("nop"))
}
