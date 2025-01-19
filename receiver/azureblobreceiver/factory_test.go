// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureblobreceiver"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestNewFactory(t *testing.T) {
	f := NewFactory()

	assert.NotNil(t, f)
}

func TestCreateTraces(t *testing.T) {
	f := NewFactory()
	ctx := context.Background()
	params := receivertest.NewNopSettings()
	receiver, err := f.CreateTraces(ctx, params, getConfig(), consumertest.NewNop())

	require.NoError(t, err)
	assert.NotNil(t, receiver)
}

func TestCreateLogs(t *testing.T) {
	f := NewFactory()
	ctx := context.Background()
	params := receivertest.NewNopSettings()
	receiver, err := f.CreateLogs(ctx, params, getConfig(), consumertest.NewNop())

	require.NoError(t, err)
	assert.NotNil(t, receiver)
}

func TestTracesAndLogsReceiversAreSame(t *testing.T) {
	f := NewFactory()
	ctx := context.Background()
	params := receivertest.NewNopSettings()
	config := getConfig()
	logsReceiver, err := f.CreateLogs(ctx, params, config, consumertest.NewNop())
	require.NoError(t, err)

	tracesReceiver, err := f.CreateTraces(ctx, params, config, consumertest.NewNop())
	require.NoError(t, err)

	assert.Equal(t, logsReceiver, tracesReceiver)
}

func getConfig() component.Config {
	return &Config{
		Authentication:   "connection_string",
		ConnectionString: goodConnectionString,
		Logs:             LogsConfig{ContainerName: logsContainerName},
		Traces:           TracesConfig{ContainerName: tracesContainerName},
	}
}
