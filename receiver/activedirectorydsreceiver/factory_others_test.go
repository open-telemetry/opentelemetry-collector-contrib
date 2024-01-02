// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows
// +build !windows

package activedirectorydsreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver"
)

func TestCreateMetricsReceiver(t *testing.T) {
	t.Parallel()

	recv, err := createMetricsReceiver(context.Background(), receiver.CreateSettings{}, &Config{}, &consumertest.MetricsSink{})
	require.Nil(t, recv)
	require.ErrorIs(t, err, errReceiverNotSupported)
}
