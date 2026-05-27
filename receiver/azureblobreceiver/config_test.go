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
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/otelcol/otelcoltest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureblobreceiver/internal/metadata"
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

	receiver := cfg.Receivers[component.NewID(metadata.Type)]
	assert.NoError(t, componenttest.CheckConfigStruct(receiver))
	assert.Equal(
		t,
		&Config{
			Authentication:   ConnectionStringAuth,
			ConnectionString: goodConnectionString,
			Logs:             LogsConfig{ContainerName: logsContainerName, Encoding: EncodingOTLPJSON},
			Traces:           TracesConfig{ContainerName: tracesContainerName, Encoding: EncodingOTLPJSON},
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
			Logs:              LogsConfig{ContainerName: logsContainerName, Encoding: EncodingOTLPJSON},
			Traces:            TracesConfig{ContainerName: tracesContainerName, Encoding: EncodingOTLPJSON},
			Cloud:             defaultCloud,
		},
		receiver)
}

func TestMissingConnectionString(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	err := xconfmap.Validate(cfg)
	assert.EqualError(t, err, `"ConnectionString" is not specified in config`)
}

func TestMissingServicePrincipalCredentials(t *testing.T) {
	var err error
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	cfg.(*Config).Authentication = ServicePrincipalAuth
	err = xconfmap.Validate(cfg)
	assert.EqualError(t, err, `"TenantID" is not specified in config; "ClientID" is not specified in config; "ClientSecret" is not specified in config; "StorageAccountURL" is not specified in config`)
}

func TestInvalidEncoding(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.ConnectionString = goodConnectionString
	// Values that are neither a built-in encoding nor a syntactically valid
	// encoding extension ID are rejected during validation.
	cfg.Logs.Encoding = "not a valid id"
	cfg.Traces.Encoding = "also not valid"
	err := xconfmap.Validate(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `logs.encoding "not a valid id" is not a supported built-in encoding`)
	assert.Contains(t, err.Error(), `traces.encoding "also not valid" is not a supported built-in encoding`)
}

func TestEncodingExtensionIDAcceptedByValidation(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.ConnectionString = goodConnectionString
	// An encoding extension ID is syntactically valid; its existence is only
	// checked when the receiver starts.
	cfg.Logs.Encoding = "myencoding"
	cfg.Traces.Encoding = "myencoding/traces"
	require.NoError(t, xconfmap.Validate(cfg))
}

func TestBlankEncoding(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.ConnectionString = goodConnectionString
	// A blank encoding is neither a built-in nor a valid extension ID, since an
	// empty component ID is rejected.
	cfg.Logs.Encoding = ""
	cfg.Traces.Encoding = ""
	err := xconfmap.Validate(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `logs.encoding "" is not a supported built-in encoding`)
	assert.Contains(t, err.Error(), `traces.encoding "" is not a supported built-in encoding`)
	assert.Contains(t, err.Error(), "id must not be empty")
}
