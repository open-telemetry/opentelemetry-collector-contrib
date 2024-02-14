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

func TestCreateTracesReceiver(t *testing.T) {
	f := NewFactory()
	ctx := context.Background()
	params := receivertest.NewNopCreateSettings()
	receiver, err := f.CreateTracesReceiver(ctx, params, getConfig(), consumertest.NewNop())

	require.NoError(t, err)
	assert.NotNil(t, receiver)
}

func TestCreateLogsReceiver(t *testing.T) {
	f := NewFactory()
	ctx := context.Background()
	params := receivertest.NewNopCreateSettings()
	receiver, err := f.CreateLogsReceiver(ctx, params, getConfig(), consumertest.NewNop())

	require.NoError(t, err)
	assert.NotNil(t, receiver)
}

func TestTracesAndLogsReceiversAreSame(t *testing.T) {
	f := NewFactory()
	ctx := context.Background()
	params := receivertest.NewNopCreateSettings()
	config := getConfig()
	logsReceiver, err := f.CreateLogsReceiver(ctx, params, config, consumertest.NewNop())
	require.NoError(t, err)

	tracesReceiver, err := f.CreateTracesReceiver(ctx, params, config, consumertest.NewNop())
	require.NoError(t, err)

	assert.Equal(t, logsReceiver, tracesReceiver)
}

func getConfig() component.Config {
	return &Config{
		ConnectionString: goodConnectionString,
		Logs:             LogsConfig{ContainerName: logsContainerName},
		Traces:           TracesConfig{ContainerName: tracesContainerName},
	}
}
