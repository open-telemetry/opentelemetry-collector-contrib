// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscontainerinsightreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestNewFactory(t *testing.T) {
	c := NewFactory()
	assert.NotNil(t, c)
}

func TestCreateMetricsReceiver(t *testing.T) {
	metricsReceiver, _ := createMetricsReceiver(
		context.Background(),
		receivertest.NewNopCreateSettings(),
		createDefaultConfig(),
		consumertest.NewNop(),
	)

	require.NotNil(t, metricsReceiver)
}
