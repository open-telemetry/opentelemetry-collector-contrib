// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudfoundryreceiver

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudfoundryreceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id           component.ID
		expected     component.Config
		errorMessage string
	}{
		{
			id: component.NewIDWithName(metadata.Type, "one"),
			expected: &Config{
				RLPGateway: RLPGatewayConfig{
					HTTPClientSettings: confighttp.HTTPClientSettings{
						Endpoint: "https://log-stream.sys.example.internal",
						TLSSetting: configtls.TLSClientSetting{
							InsecureSkipVerify: true,
						},
						Timeout: time.Second * 20,
					},
					ShardID: "otel-test",
				},
				UAA: UAAConfig{
					LimitedHTTPClientSettings: LimitedHTTPClientSettings{
						Endpoint: "https://uaa.sys.example.internal",
						TLSSetting: LimitedTLSClientSetting{
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
	}
	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			if tt.expected == nil {
				assert.EqualError(t, component.ValidateConfig(cfg), tt.errorMessage)
				return
			}
			assert.NoError(t, component.ValidateConfig(cfg))
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
	configuration.UAA.Endpoint = "https://[invalid"
	require.Error(t, configuration.Validate())
}

func TestHTTPConfigurationStructConsistency(t *testing.T) {
	// LimitedHTTPClientSettings must have the same structure as HTTPClientSettings, but without the fields that the UAA
	// library does not support.
	checkTypeFieldMatch(t, "Endpoint", reflect.TypeOf(LimitedHTTPClientSettings{}), reflect.TypeOf(confighttp.HTTPClientSettings{}))
	checkTypeFieldMatch(t, "TLSSetting", reflect.TypeOf(LimitedHTTPClientSettings{}), reflect.TypeOf(confighttp.HTTPClientSettings{}))
	checkTypeFieldMatch(t, "InsecureSkipVerify", reflect.TypeOf(LimitedTLSClientSetting{}), reflect.TypeOf(configtls.TLSClientSetting{}))
}

func loadSuccessfulConfig(t *testing.T) *Config {
	configuration := &Config{
		RLPGateway: RLPGatewayConfig{
			HTTPClientSettings: confighttp.HTTPClientSettings{
				Endpoint: "https://log-stream.sys.example.internal",
				Timeout:  time.Second * 20,
				TLSSetting: configtls.TLSClientSetting{
					InsecureSkipVerify: true,
				},
			},
			ShardID: "otel-test",
		},
		UAA: UAAConfig{
			LimitedHTTPClientSettings: LimitedHTTPClientSettings{
				Endpoint: "https://uaa.sys.example.internal",
				TLSSetting: LimitedTLSClientSetting{
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
	require.Equal(t, localField.Tag, standardField.Tag, "field %s tag match", fieldName)
}
