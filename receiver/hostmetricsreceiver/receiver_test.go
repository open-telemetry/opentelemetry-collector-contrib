// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hostmetricsreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/metadata"
)

func TestReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	settings := receivertest.NewNopSettings(metadata.Type)
	sink := new(consumertest.LogsSink)
	logs, err := factory.CreateLogs(t.Context(), settings, cfg, sink)
	require.NoError(t, err)
	assert.NoError(t, logs.Start(t.Context(), componenttest.NewNopHost()))
	assert.NoError(t, logs.Shutdown(t.Context()))

	allLogs := sink.AllLogs()
	require.Len(t, allLogs, 1)
}
