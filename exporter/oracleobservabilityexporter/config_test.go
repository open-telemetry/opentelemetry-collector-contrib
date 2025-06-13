// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
// Copyright © 2025, Oracle and/or its affiliates.

package oracleobservabilityexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/oracleobservabilityexporter"

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

var oracleobservability = component.MustNewType("oracleobservability")

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	id := component.NewIDWithName(oracleobservability, "1")

	cfg := createDefaultConfig()

	// Load the sub-config from the config map
	sub, err := cm.Sub(id.String())
	require.NoError(t, err)

	// Unmarshal the loaded configuration into the config object
	require.NoError(t, sub.Unmarshal(cfg))

	// Validate that the configuration is correct
	assert.NoError(t, xconfmap.Validate(cfg))

	expected := &Config{
		TimeoutConfig:  exporterhelper.TimeoutConfig{Timeout: 0},
		QueueConfig:    exporterhelper.QueueBatchConfig{Enabled: true, NumConsumers: 10, QueueSize: 1000, BlockOnOverflow: true, WaitForResult: false, Sizer: exporterhelper.RequestSizerTypeRequests},
		BackOffConfig:  configretry.BackOffConfig{Enabled: true, InitialInterval: 5 * time.Second, RandomizationFactor: 0.5, Multiplier: 1.5, MaxInterval: 30 * time.Second, MaxElapsedTime: 0},
		AuthType:       "config_file",
		NamespaceName:  "example-namespace",
		LogGroupID:     "example-loggroup-id",
		ConfigFilePath: "path/to/config/file",
		ConfigProfile:  "default-profile",
	}

	// Compare the unmarshalled configuration with the expected values
	assert.Equal(t, expected, cfg)
}

func TestConfigValidate(t *testing.T) {
	t.Parallel()

	validConfig := &Config{
		AuthType:      ConfigFile,
		NamespaceName: "test-namespace",
		LogGroupID:    "test-log-group",
		ConfigProfile: "test-profile",
		OciConfiguration: OciConfig{
			FingerPrint: "test-fingerprint",
			PrivateKey:  configopaque.String("test-private-key"),
			Tenancy:     "test-tenancy",
			Region:      "test-region",
			User:        "test-user",
		},
	}

	tests := []struct {
		name        string
		config      *Config
		expectedErr string
	}{
		{
			name:        "Nil config",
			config:      nil,
			expectedErr: "missing configuration, you must provide a valid configuration for the Oracle Observability exporter",
		},
		{
			name: "Missing namespace",
			config: &Config{
				AuthType:   ConfigFile,
				LogGroupID: "test-log-group",
			},
			expectedErr: "'namespace' is a required field. You may find using OCI Console under Logging Analytics → Administration → Service",
		},
		{
			name: "Missing log group",
			config: &Config{
				AuthType:      ConfigFile,
				NamespaceName: "test-namespace",
			},
			expectedErr: "'log_group_id' is a required field",
		},
		{
			name: "Invalid auth type",
			config: &Config{
				AuthType:      "invalid",
				NamespaceName: "test-namespace",
				LogGroupID:    "test-log-group",
			},
			expectedErr: "invalid 'auth_type', supported values are 'config_file' and 'instance_principal'",
		},
		{
			name:        "Valid config",
			config:      validConfig,
			expectedErr: "",
		},
		{
			name: "Instance principal with OCI config",
			config: &Config{
				AuthType:      InstancePrincipal,
				NamespaceName: "test-namespace",
				LogGroupID:    "test-log-group",
				OciConfiguration: OciConfig{
					FingerPrint: "test",
				},
			},
			expectedErr: "'oci_config' field is not applicable when 'auth_type' is set to 'instance_principal'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tt.expectedErr)
			}
		})
	}
}

func TestOciConfigValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		config      *Config
		expectedErr string
	}{
		{
			name: "Missing fingerprint",
			config: &Config{
				AuthType:      ConfigFile,
				NamespaceName: "test-namespace",
				LogGroupID:    "test-log-group",
				ConfigProfile: "test-profile",
				OciConfiguration: OciConfig{
					PrivateKey: "test-key",
					Tenancy:    "test-tenancy",
					Region:     "test-region",
					User:       "test-user",
				},
			},
			expectedErr: "'fingerprint' can not be empty",
		},
		{
			name: "Missing private key",
			config: &Config{
				AuthType:      ConfigFile,
				NamespaceName: "test-namespace",
				LogGroupID:    "test-log-group",
				ConfigProfile: "test-profile",
				OciConfiguration: OciConfig{
					FingerPrint: "test-fingerprint",
					Tenancy:     "test-tenancy",
					Region:      "test-region",
					User:        "test-user",
				},
			},
			expectedErr: "'private_key' can not be empty",
		},
		{
			name: "Missing region",
			config: &Config{
				AuthType:      ConfigFile,
				NamespaceName: "test-namespace",
				LogGroupID:    "test-log-group",
				ConfigProfile: "test-profile",
				OciConfiguration: OciConfig{
					FingerPrint: "test-fingerprint",
					PrivateKey:  "test-key",
					Tenancy:     "test-tenancy",
					User:        "test-user",
				},
			},
			expectedErr: "'region' can not be empty",
		},
		{
			name: "Missing tenancy",
			config: &Config{
				AuthType:      ConfigFile,
				NamespaceName: "test-namespace",
				LogGroupID:    "test-log-group",
				ConfigProfile: "test-profile",
				OciConfiguration: OciConfig{
					FingerPrint: "test-fingerprint",
					PrivateKey:  "test-key",
					Region:      "test-region",
					User:        "test-user",
				},
			},
			expectedErr: "'tenancy' can not be empty",
		},
		{
			name: "Missing user",
			config: &Config{
				AuthType:      ConfigFile,
				NamespaceName: "test-namespace",
				LogGroupID:    "test-log-group",
				ConfigProfile: "test-profile",
				OciConfiguration: OciConfig{
					FingerPrint: "test-fingerprint",
					PrivateKey:  "test-key",
					Tenancy:     "test-tenancy",
					Region:      "test-region",
				},
			},
			expectedErr: "'user' can not be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			assert.EqualError(t, err, tt.expectedErr)
		})
	}
}
