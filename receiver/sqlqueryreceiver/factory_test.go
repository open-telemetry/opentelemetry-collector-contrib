// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlqueryreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	_, err := factory.CreateMetrics(
		context.Background(),
		receivertest.NewNopSettings(),
		factory.CreateDefaultConfig(),
		consumertest.NewNop(),
	)
	require.NoError(t, err)
	_, err = factory.CreateLogs(
		context.Background(),
		receivertest.NewNopSettings(),
		factory.CreateDefaultConfig(),
		consumertest.NewNop(),
	)
	require.NoError(t, err)
}
