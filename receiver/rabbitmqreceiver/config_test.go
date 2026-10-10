// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package rabbitmqreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/rabbitmqreceiver"

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/rabbitmqreceiver/internal/metadata"
)

func TestValidate(t *testing.T) {
	clientConfigInvalid := confighttp.NewDefaultClientConfig()
	clientConfigInvalid.Endpoint = "invalid://endpoint:  12efg"

	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = defaultEndpoint

	testCases := []struct {
		desc        string
		cfg         *Config
		expectedErr error
	}{
		{
			desc: "missing username, password, and invalid endpoint",
			cfg: &Config{
				ClientConfig: clientConfigInvalid,
			},
			expectedErr: errors.Join(
				errMissingUsername,
				errMissingPassword,
				fmt.Errorf("%w: %s", errInvalidEndpoint, `parse "invalid://endpoint:  12efg": invalid port ":  12efg" after host`),
			),
		},
		{
			desc: "missing password and invalid endpoint",
			cfg: &Config{
				Username:     "otelu",
				ClientConfig: clientConfigInvalid,
			},
			expectedErr: errors.Join(
				errMissingPassword,
				fmt.Errorf("%w: %s", errInvalidEndpoint, `parse "invalid://endpoint:  12efg": invalid port ":  12efg" after host`),
			),
		},
		{
			desc: "missing username and invalid endpoint",
			cfg: &Config{
				Password:     "otelp",
				ClientConfig: clientConfigInvalid,
			},
			expectedErr: errors.Join(
				errMissingUsername,
				fmt.Errorf("%w: %s", errInvalidEndpoint, `parse "invalid://endpoint:  12efg": invalid port ":  12efg" after host`),
			),
		},
		{
			desc: "invalid endpoint",
			cfg: &Config{
				Username:     "otelu",
				Password:     "otelp",
				ClientConfig: clientConfigInvalid,
			},
			expectedErr: errors.Join(
				fmt.Errorf("%w: %s", errInvalidEndpoint, `parse "invalid://endpoint:  12efg": invalid port ":  12efg" after host`),
			),
		},
		{
			desc: "valid config",
			cfg: &Config{
				Username:     "otelu",
				Password:     "otelp",
				ClientConfig: clientConfig,
			},
			expectedErr: nil,
		},
		{
			desc: "valid queues extract arguments config",
			cfg: &Config{
				Username:     "otelu",
				Password:     "otelp",
				ClientConfig: clientConfig,
				Queues: QueuesConfig{
					Extract: ExtractConfig{
						Arguments: []FieldExtractConfig{
							{TagName: "owner", Key: "owner"},
							{TagName: "$1", KeyRegex: "x-(.*)"},
						},
					},
				},
			},
			expectedErr: nil,
		},
		{
			desc: "queues extract arguments rule with neither key nor key_regex",
			cfg: &Config{
				Username:     "otelu",
				Password:     "otelp",
				ClientConfig: clientConfig,
				Queues: QueuesConfig{
					Extract: ExtractConfig{
						Arguments: []FieldExtractConfig{{TagName: "owner"}},
					},
				},
			},
			expectedErr: fmt.Errorf(`invalid queues::extract::arguments rule (tag_name: "owner"): %w`, errFieldExtractKeyAmbiguous),
		},
		{
			desc: "queues extract arguments rule with both key and key_regex",
			cfg: &Config{
				Username:     "otelu",
				Password:     "otelp",
				ClientConfig: clientConfig,
				Queues: QueuesConfig{
					Extract: ExtractConfig{
						Arguments: []FieldExtractConfig{{TagName: "owner", Key: "owner", KeyRegex: "x-.*"}},
					},
				},
			},
			expectedErr: fmt.Errorf(`invalid queues::extract::arguments rule (tag_name: "owner"): %w`, errFieldExtractKeyAmbiguous),
		},
		{
			desc: "queues extract arguments rule with invalid key_regex",
			cfg: &Config{
				Username:     "otelu",
				Password:     "otelp",
				ClientConfig: clientConfig,
				Queues: QueuesConfig{
					Extract: ExtractConfig{
						Arguments: []FieldExtractConfig{{TagName: "owner", KeyRegex: "("}},
					},
				},
			},
			expectedErr: fmt.Errorf(`invalid queues::extract::arguments key_regex "(": %w`, errors.New("error parsing regexp: missing closing ): `^(?:()$`")),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			actualErr := tc.cfg.Validate()
			if tc.expectedErr != nil {
				require.EqualError(t, actualErr, tc.expectedErr.Error())
			} else {
				require.NoError(t, actualErr)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	expected := factory.CreateDefaultConfig().(*Config)
	expected.ClientConfig.Endpoint = "http://localhost:15672"
	expected.Username = "otelu"
	expected.Password = "${env:RABBITMQ_PASSWORD}"
	expected.ControllerConfig.CollectionInterval = 10 * time.Second

	require.Equal(t, expected, cfg)
}
