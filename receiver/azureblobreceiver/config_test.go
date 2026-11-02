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
	"go.opentelemetry.io/collector/confmap"
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
		receiver,
	)

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
		receiver,
	)
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name        string
		mutate      func(cfg *Config)
		expectedErr []string
		equalErr    string
	}{
		{
			name:     "missing connection string",
			mutate:   func(_ *Config) {},
			equalErr: `"ConnectionString" is not specified in config`,
		},
		{
			name: "missing service principal credentials",
			mutate: func(cfg *Config) {
				cfg.Authentication = ServicePrincipalAuth
			},
			equalErr: `"TenantID" is not specified in config; "ClientID" is not specified in config; "ClientSecret" is not specified in config; "StorageAccountURL" is not specified in config`,
		},
		{
			name: "invalid encoding",
			mutate: func(cfg *Config) {
				cfg.ConnectionString = goodConnectionString
				// Values that are neither a built-in encoding nor a syntactically valid
				// encoding extension ID are rejected during validation.
				cfg.Logs.Encoding = "not a valid id"
				cfg.Traces.Encoding = "also not valid"
			},
			expectedErr: []string{
				`logs.encoding "not a valid id" is not a supported built-in encoding`,
				`traces.encoding "also not valid" is not a supported built-in encoding`,
			},
		},
		{
			name: "encoding extension ID accepted by validation",
			mutate: func(cfg *Config) {
				cfg.ConnectionString = goodConnectionString
				// An encoding extension ID is syntactically valid; its existence is only
				// checked when the receiver starts.
				cfg.Logs.Encoding = "myencoding"
				cfg.Traces.Encoding = "myencoding/traces"
			},
		},
		{
			name: "blank encoding",
			mutate: func(cfg *Config) {
				cfg.ConnectionString = goodConnectionString
				// A blank encoding is neither a built-in nor a valid extension ID, since an
				// empty component ID is rejected.
				cfg.Logs.Encoding = ""
				cfg.Traces.Encoding = ""
			},
			expectedErr: []string{
				`logs.encoding "" is not a supported built-in encoding`,
				`traces.encoding "" is not a supported built-in encoding`,
			},
		},
		{
			name: "invalid compression",
			mutate: func(cfg *Config) {
				cfg.ConnectionString = goodConnectionString
				cfg.Compression = "snappy"
			},
			expectedErr: []string{
				`compression "snappy" is not supported`,
			},
		},
		{
			name: "valid compression none",
			mutate: func(cfg *Config) {
				cfg.ConnectionString = goodConnectionString
				cfg.Compression = CompressionNone
			},
		},
		{
			name: "valid compression gzip",
			mutate: func(cfg *Config) {
				cfg.ConnectionString = goodConnectionString
				cfg.Compression = CompressionGzip
			},
		},
		{
			name: "valid compression auto",
			mutate: func(cfg *Config) {
				cfg.ConnectionString = goodConnectionString
				cfg.Compression = CompressionAuto
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewFactory().CreateDefaultConfig().(*Config)
			tt.mutate(cfg)
			err := confmap.Validate(cfg)
			switch {
			case tt.equalErr != "":
				assert.EqualError(t, err, tt.equalErr)
			case len(tt.expectedErr) > 0:
				require.Error(t, err)
				for _, expected := range tt.expectedErr {
					assert.Contains(t, err.Error(), expected)
				}
			default:
				require.NoError(t, err)
			}
		})
	}
}
