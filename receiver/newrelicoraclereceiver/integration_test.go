// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package newrelicoraclereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver"

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestNewRelicOracleReceiverIntegration(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	// Use fake datasource for testing
	cfg.DataSource = "oracle://fake:fake@localhost:1521/XE"

	set := receivertest.NewNopSettings()
	consumer := consumertest.NewNop()

	// Create receiver with fake client
	receiver, err := createReceiverFunc(func(dataSourceName string) (*sql.DB, error) {
		return nil, nil // We'll use fake client
	}, newFakeDbClient)(context.Background(), set, cfg, consumer)

	require.NoError(t, err)
	require.NotNil(t, receiver)

	// Test start and shutdown
	err = receiver.Start(context.Background(), componenttest.NewNopHost())
	assert.NoError(t, err)

	// Let it run briefly
	time.Sleep(100 * time.Millisecond)

	err = receiver.Shutdown(context.Background())
	assert.NoError(t, err)
}
