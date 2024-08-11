// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hostmetricsreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	settings := receivertest.NewNopSettings()
	sink := new(consumertest.LogsSink)
	logs, err := factory.CreateLogsReceiver(context.Background(), settings, cfg, sink)
	require.NoError(t, err)
	assert.NoError(t, logs.Start(context.Background(), componenttest.NewNopHost()))
	assert.NoError(t, logs.Shutdown(context.Background()))

	allLogs := sink.AllLogs()
	require.Len(t, allLogs, 1)
}
