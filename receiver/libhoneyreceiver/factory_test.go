// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package libhoneyreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/libhoneyreceiver/internal/metadata"
)

func TestCreateTracesReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	set := receivertest.NewNopSettings(metadata.Type)
	tReceiver, err := factory.CreateTraces(t.Context(), set, cfg, consumertest.NewNop())

	assert.NoError(t, err, "receiver creation failed")
	assert.NotNil(t, tReceiver, "receiver creation failed")

	assert.NoError(t, tReceiver.Start(t.Context(), componenttest.NewNopHost()))
	assert.NoError(t, tReceiver.Shutdown(t.Context()))
}

func TestCreateLogsReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	set := receivertest.NewNopSettings(metadata.Type)
	lReceiver, err := factory.CreateLogs(t.Context(), set, cfg, consumertest.NewNop())

	assert.NoError(t, err, "receiver creation failed")
	assert.NotNil(t, lReceiver, "receiver creation failed")

	assert.NoError(t, lReceiver.Start(t.Context(), componenttest.NewNopHost()))
	assert.NoError(t, lReceiver.Shutdown(t.Context()))
}

func TestType(t *testing.T) {
	factory := NewFactory()
	assert.Equal(t, metadata.Type, factory.Type())
}
