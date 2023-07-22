// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestDatadogReceiver_Lifecycle(t *testing.T) {

	factory := NewFactory()
	ddr, err := factory.CreateTracesReceiver(context.Background(), receivertest.NewNopCreateSettings(), factory.CreateDefaultConfig(), consumertest.NewNop())
	assert.NoError(t, err, "Receiver should be created")

	err = ddr.Start(context.Background(), componenttest.NewNopHost())
	assert.NoError(t, err, "Server should start")

	err = ddr.Shutdown(context.Background())
	assert.NoError(t, err, "Server should stop")
}
