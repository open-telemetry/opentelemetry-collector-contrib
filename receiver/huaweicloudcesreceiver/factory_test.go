// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package huaweicloudcesreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/huaweicloudcesreceiver/internal/metadata"
)

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	assert.NotNil(t, factory)
	assert.Equal(t, component.MustNewType("huaweicloudcesreceiver"), factory.Type())
}

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	config := factory.CreateDefaultConfig()
	assert.NotNil(t, config)
	assert.NoError(t, componenttest.CheckConfigStruct(config))
}

func TestCreateMetricsReceiver(t *testing.T) {
	factory := NewFactory()
	config := factory.CreateDefaultConfig()

	rConfig := config.(*Config)
	rConfig.CollectionInterval = 60 * time.Second
	rConfig.InitialDelay = time.Second

	nextConsumer := new(consumertest.MetricsSink)
	receiver, err := factory.CreateMetrics(t.Context(), receivertest.NewNopSettings(metadata.Type), config, nextConsumer)
	assert.NoError(t, err)
	assert.NotNil(t, receiver)
}

func TestConfigMarshalRoundTripPreservesSessionConfig(t *testing.T) {
	config := NewFactory().CreateDefaultConfig().(*Config)
	in := confmap.NewFromStringMap(map[string]any{
		"collection_interval": "1m",
		"endpoint":            "https://example.invalid",
		"access_key":          "ak",
		"secret_key":          "sk",
		"proxy_address":       "http://proxy.invalid:8080",
		"proxy_user":          "u",
		"proxy_password":      "p",
		"project_id":          "proj",
		"region_id":           "eu-test-1",
		"period":              300,
		"filter":              "max",
	})
	require.NoError(t, in.Unmarshal(config))

	out := confmap.New()
	require.NoError(t, out.Marshal(config))

	roundTrip := out.ToStringMap()
	assert.Equal(t, "[REDACTED]", roundTrip["access_key"])
	assert.Equal(t, "[REDACTED]", roundTrip["secret_key"])
	assert.Equal(t, "http://proxy.invalid:8080", roundTrip["proxy_address"])
	assert.Equal(t, "u", roundTrip["proxy_user"])
	assert.Equal(t, "p", roundTrip["proxy_password"])
	assert.Equal(t, "proj", roundTrip["project_id"])
	assert.Equal(t, "eu-test-1", roundTrip["region_id"])
}
