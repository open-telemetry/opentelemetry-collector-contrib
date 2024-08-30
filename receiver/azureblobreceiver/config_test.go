// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureblobreceiver"

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/otelcol/otelcoltest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureblobreceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	factories, err := otelcoltest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[metadata.Type] = factory
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/33594
	// nolint:staticcheck
	cfg, err := otelcoltest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Len(t, cfg.Receivers, 2)

	receiver := cfg.Receivers[component.NewID(metadata.Type)]
	assert.NoError(t, componenttest.CheckConfigStruct(receiver))
	assert.Equal(
		t,
		&Config{
			Authentication:   ConnectionStringAuth,
			ConnectionString: goodConnectionString,
			Logs:             LogsConfig{ContainerName: logsContainerName},
			Traces:           TracesConfig{ContainerName: tracesContainerName},
			Cloud:            defaultCloud,
		},
		receiver)

	receiver = cfg.Receivers[component.NewIDWithName(metadata.Type, "2")].(*Config)
	assert.NoError(t, componenttest.CheckConfigStruct(receiver))
	assert.Equal(
		t,
		&Config{
			Authentication: ServicePrincipalAuth,
			ServicePrincipal: ServicePrincipalConfig{
				TenantID:     "mock-tenant-id",
				ClientID:     "mock-client-id",
				ClientSecret: "mock-client-secret",
			},
			StorageAccountURL: "https://accountName.blob.core.windows.net",
			Logs:              LogsConfig{ContainerName: logsContainerName},
			Traces:            TracesConfig{ContainerName: tracesContainerName},
			Cloud:             defaultCloud,
		},
		receiver)
}

func TestMissingConnectionString(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	err := component.ValidateConfig(cfg)
	assert.EqualError(t, err, `"ConnectionString" is not specified in config`)
}

func TestMissingServicePrincipalCredentials(t *testing.T) {
	var err error
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	cfg.(*Config).Authentication = ServicePrincipalAuth
	err = component.ValidateConfig(cfg)
	assert.EqualError(t, err, `"TenantID" is not specified in config; "ClientID" is not specified in config; "ClientSecret" is not specified in config; "StorageAccountURL" is not specified in config`)
}
