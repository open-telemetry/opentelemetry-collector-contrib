// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/otelcol/otelcoltest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	factories, err := otelcoltest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[metadata.Type] = factory
	cfg, err := otelcoltest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Len(t, cfg.Receivers, 2)

	r0 := cfg.Receivers[component.NewID(metadata.Type)]
	assert.Equal(t, "Endpoint=sb://namespace.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=superSecret1234=;EntityPath=hubName", r0.(*Config).Connection)
	assert.Empty(t, r0.(*Config).Offset)
	assert.Empty(t, r0.(*Config).Partition)
	assert.Equal(t, defaultLogFormat, logFormat(r0.(*Config).Format))
	assert.False(t, r0.(*Config).ApplySemanticConventions)

	r1 := cfg.Receivers[component.NewIDWithName(metadata.Type, "all")]
	assert.Equal(t, "Endpoint=sb://namespace.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=superSecret1234=;EntityPath=hubName", r1.(*Config).Connection)
	assert.Equal(t, "1234-5566", r1.(*Config).Offset)
	assert.Equal(t, "foo", r1.(*Config).Partition)
	assert.Equal(t, rawLogFormat, logFormat(r1.(*Config).Format))
	assert.True(t, r1.(*Config).ApplySemanticConventions)
}

func TestMissingConnection(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	err := xconfmap.Validate(cfg)
	assert.EqualError(t, err, "missing connection")
}

func TestInvalidConnectionString(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	cfg.(*Config).Connection = "foo"
	err := xconfmap.Validate(cfg)
	assert.EqualError(t, err, "failed parsing connection string due to unmatched key value separated by '='")
}

func TestIsValidFormat(t *testing.T) {
	for _, format := range []logFormat{defaultLogFormat, rawLogFormat, azureLogFormat} {
		assert.True(t, isValidFormat(string(format)))
	}
	assert.False(t, isValidFormat("invalid-format"))
}

func TestInvalidFormat(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	cfg.(*Config).Connection = "Endpoint=sb://namespace.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=superSecret1234=;EntityPath=hubName"
	cfg.(*Config).Format = "invalid"
	err := xconfmap.Validate(cfg)
	assert.ErrorContains(t, err, "invalid format; must be one of")
}

func TestOffsetWithoutPartition(t *testing.T) {
	cfg := NewFactory().CreateDefaultConfig().(*Config)
	cfg.Connection = "Endpoint=sb://namespace.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=superSecret1234=;EntityPath=hubName"
	cfg.Offset = "foo"
	assert.ErrorContains(t, cfg.Validate(), "cannot use 'offset' without 'partition'")
}
