// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudfoundryreceiver

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudfoundryreceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = "https://log-stream.sys.example.internal"
	clientConfig.TLS = configtls.ClientConfig{
		InsecureSkipVerify: true,
	}
	clientConfig.Timeout = time.Second * 20

	tests := []struct {
		id           component.ID
		expected     component.Config
		errorMessage string
	}{
		{
			id: component.NewIDWithName(metadata.Type, "one"),
			expected: &Config{
				RLPGateway: RLPGatewayConfig{
					ClientConfig: clientConfig,
					ShardID:      "otel-test",
				},
				UAA: UAAConfig{
					LimitedClientConfig: LimitedClientConfig{
						Endpoint: "https://uaa.sys.example.internal",
						TLS: LimitedTLSClientSetting{
							InsecureSkipVerify: true,
						},
					},
					Username: "admin",
					Password: "test",
				},
			},
		},
		{
			id:           component.NewIDWithName(metadata.Type, "empty"),
			errorMessage: "UAA password not specified",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "invalid"),
			errorMessage: "failed to parse rlp_gateway.endpoint as url: parse \"https://[invalid\": missing ']' in host",
		},
		{
			id: component.NewIDWithName(metadata.Type, "shardidnotdefined"),
			expected: &Config{
				RLPGateway: RLPGatewayConfig{
					ClientConfig: clientConfig,
					ShardID:      "opentelemetry",
				},
				UAA: UAAConfig{
					LimitedClientConfig: LimitedClientConfig{
						Endpoint: "https://uaa.sys.example.internal",
						TLS: LimitedTLSClientSetting{
							InsecureSkipVerify: true,
						},
					},
					Username: "admin",
					Password: "test",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			if tt.expected == nil {
				assert.EqualError(t, xconfmap.Validate(cfg), tt.errorMessage)
				return
			}
			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestInvalidConfigValidation(t *testing.T) {
	configuration := loadSuccessfulConfig(t)
	configuration.RLPGateway.Endpoint = "https://[invalid"
	require.Error(t, configuration.Validate())

	configuration = loadSuccessfulConfig(t)
	configuration.UAA.Username = ""
	require.Error(t, configuration.Validate())

	configuration = loadSuccessfulConfig(t)
	configuration.UAA.Password = ""
	require.Error(t, configuration.Validate())

	configuration = loadSuccessfulConfig(t)
	configuration.RLPGateway.ShardID = ""
	require.Error(t, configuration.Validate())

	configuration = loadSuccessfulConfig(t)
	configuration.UAA.Endpoint = "https://[invalid"
	require.Error(t, configuration.Validate())
}

func TestHTTPConfigurationStructConsistency(t *testing.T) {
	// LimitedClientConfig must have the same structure as ClientConfig, but without the fields that the UAA
	// library does not support.
	checkTypeFieldMatch(t, "Endpoint", reflect.TypeOf(LimitedClientConfig{}), reflect.TypeOf(confighttp.NewDefaultClientConfig()))
	checkTypeFieldMatch(t, "TLS", reflect.TypeOf(LimitedClientConfig{}), reflect.TypeOf(confighttp.NewDefaultClientConfig()))
	checkTypeFieldMatch(t, "InsecureSkipVerify", reflect.TypeOf(LimitedTLSClientSetting{}), reflect.TypeOf(configtls.ClientConfig{}))
}

func loadSuccessfulConfig(t *testing.T) *Config {
	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = "https://log-stream.sys.example.internal"
	clientConfig.Timeout = time.Second * 20
	clientConfig.TLS = configtls.ClientConfig{
		InsecureSkipVerify: true,
	}
	configuration := &Config{
		RLPGateway: RLPGatewayConfig{
			ClientConfig: clientConfig,
			ShardID:      "otel-test",
		},
		UAA: UAAConfig{
			LimitedClientConfig: LimitedClientConfig{
				Endpoint: "https://uaa.sys.example.internal",
				TLS: LimitedTLSClientSetting{
					InsecureSkipVerify: true,
				},
			},
			Username: "admin",
			Password: "test",
		},
	}

	require.NoError(t, configuration.Validate())
	return configuration
}

func checkTypeFieldMatch(t *testing.T, fieldName string, localType reflect.Type, standardType reflect.Type) {
	localField, localFieldPresent := localType.FieldByName(fieldName)
	standardField, standardFieldPresent := standardType.FieldByName(fieldName)

	require.True(t, localFieldPresent, "field %s present in local type", fieldName)
	require.True(t, standardFieldPresent, "field %s present in standard type", fieldName)

	// Check that the mapstructure tag is not empty
	require.GreaterOrEqual(t, len(strings.Split(localField.Tag.Get("mapstructure"), ",")), 1)

	// Check that the configuration key names are the same, ignoring other tags like omitempty.
	require.Equal(t, localField.Tag.Get("mapstructure")[0], standardField.Tag.Get("mapstructure")[0], "field %s tag match", fieldName)
}
