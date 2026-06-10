// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package activedirectoryinvreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestCreateLogsReceiver(t *testing.T) {
	f := NewFactory()
	cfg := createDefaultConfig().(*ADConfig)
	recv, err := f.CreateLogs(
		context.Background(),
		receivertest.NewNopSettings(f.Type()),
		cfg,
		&consumertest.LogsSink{},
	)
	require.Nil(t, recv)
	require.ErrorIs(t, err, errReceiverNotSupported)
}
