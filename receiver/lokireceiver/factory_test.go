// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lokireceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/lokireceiver/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	cfg.(*Config).GRPC = &configgrpc.ServerConfig{
		NetAddr: confignet.AddrConfig{
			Endpoint:  defaultGRPCEndpoint,
			Transport: confignet.TransportTypeTCP,
		},
	}
	set := receivertest.NewNopSettings(metadata.Type)
	receiver, err := factory.CreateLogs(context.Background(), set, cfg, consumertest.NewNop())
	assert.NoError(t, err, "receiver creation failed")
	assert.NotNil(t, receiver, "receiver creation failed")
}
