// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dnscheckreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dnscheckreceiver/internal/metadata"
)

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	require.NotNil(t, factory)
	assert.Equal(t, "dns_check", factory.Type().String())
}

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	require.NotNil(t, cfg)

	config, ok := cfg.(*Config)
	require.True(t, ok)
	assert.Empty(t, config.DNSServers)
	assert.Empty(t, config.Hostnames)
}

func TestCreateMetricsReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	// Populate config to make it valid.
	config := cfg.(*Config)
	config.DNSServers = []string{"8.8.8.8"}
	config.Hostnames = []HostnameConfig{{Name: "example.com", RecordType: "A"}}
	config.Timeout = 5 * time.Second

	set := receivertest.NewNopSettings(metadata.Type)
	consumer := consumertest.NewNop()

	// For skeleton, we expect a no-op receiver and no error
	receiver, err := factory.CreateMetrics(t.Context(), set, cfg, consumer)
	assert.NotNil(t, receiver)
	assert.NoError(t, err)
}

func TestFactoryCanBeUsed(t *testing.T) {
	factory := NewFactory()
	err := componenttest.CheckConfigStruct(factory.CreateDefaultConfig())
	require.NoError(t, err)
}
