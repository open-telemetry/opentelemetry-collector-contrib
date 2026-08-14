// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package netflowreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/netflowreceiver/internal/metadata"
)

func TestCreateValidDefaultReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	set := receivertest.NewNopSettings(metadata.Type)
	receiver, err := factory.CreateLogs(t.Context(), set, cfg, consumertest.NewNop())
	assert.NoError(t, err, "receiver creation failed")
	assert.NotNil(t, receiver, "receiver creation failed")
	assert.NotNil(t, receiver.(*netflowReceiver).udpReceiver)
}

func TestBuildDecodeFuncWithMapping(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Mapping = "testdata/mapping.yaml"
	nr := &netflowReceiver{config: *cfg, logger: zap.NewNop()}
	decodeFunc, err := nr.buildDecodeFunc()
	require.NoError(t, err)
	require.NotNil(t, decodeFunc)
}

func TestBuildDecodeFuncWithBadMapping(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Mapping = "testdata/does_not_exist.yaml"
	nr := &netflowReceiver{config: *cfg, logger: zap.NewNop()}
	_, err := nr.buildDecodeFunc()
	require.Error(t, err)
}
