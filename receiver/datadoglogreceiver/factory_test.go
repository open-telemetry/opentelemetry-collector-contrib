// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadoglogreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadoglogreceiver"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestCreateReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	cfg.(*Config).Endpoint = "http://localhost:0"

	tReceiver, err := factory.CreateLogsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, tReceiver, "receiver creation failed")
}
