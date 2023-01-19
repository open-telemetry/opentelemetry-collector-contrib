// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cloudflarereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudflarereceiver"

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
)

func TestValidate(t *testing.T) {
	cases := []struct {
		name        string
		config      Config
		expectedErr error
	}{
		{
			name: "Valid Config with auth email and key",
			config: Config{
				PollInterval: defaultPollInterval,
				Zone:         "023e105f4ecef8ad9ca31a8372d0c353",
				Auth: &Auth{
					XAuthKey:   "abc123",
					XAuthEmail: "email@email.com",
				},
				Logs: &LogsConfig{
					Sample: float32(defaultSampleRate),
					Count:  defaultCount,
					Fields: defaultFields,
				},
			},
			expectedErr: nil,
		},
		{
			name: "Valid Config with auth token",
			config: Config{
				PollInterval: defaultPollInterval,
				Zone:         "023e105f4ecef8ad9ca31a8372d0c353",
				Auth: &Auth{
					APIToken: "abc123",
				},
				Logs: &LogsConfig{
					Sample: float32(defaultSampleRate),
					Count:  defaultCount,
					Fields: defaultFields,
				},
			},
			expectedErr: nil,
		},
		{
			name: "Invalid Config, no Zone",
			config: Config{
				PollInterval: defaultPollInterval,
				Auth: &Auth{
					XAuthKey:   "abc123",
					XAuthEmail: "email@email.com",
				},
				Logs: &LogsConfig{
					Sample: float32(defaultSampleRate),
					Count:  defaultCount,
					Fields: defaultFields,
				},
			},
			expectedErr: errNoZone,
		},
		{
			name: "Invalid Config, no poll interval",
			config: Config{
				Zone: "023e105f4ecef8ad9ca31a8372d0c353",
				Auth: &Auth{
					XAuthKey:   "abc123",
					XAuthEmail: "email@email.com",
				},
				Logs: &LogsConfig{
					Sample: float32(defaultSampleRate),
					Count:  defaultCount,
					Fields: defaultFields,
				},
			},
			expectedErr: errInvalidPollInterval,
		},
		{
			name: "Invalid Config, no auth key",
			config: Config{
				Zone:         "023e105f4ecef8ad9ca31a8372d0c353",
				PollInterval: defaultPollInterval,
				Auth: &Auth{
					XAuthEmail: "email@email.com",
				},
				Logs: &LogsConfig{
					Sample: float32(defaultSampleRate),
					Count:  defaultCount,
					Fields: defaultFields,
				},
			},
			expectedErr: errInvalidAuthenticationConfigured,
		},
		{
			name: "Invalid Config, no auth email",
			config: Config{
				Zone:         "023e105f4ecef8ad9ca31a8372d0c353",
				PollInterval: defaultPollInterval,
				Auth: &Auth{
					XAuthKey: "abc123",
				},
				Logs: &LogsConfig{
					Sample: float32(defaultSampleRate),
					Count:  defaultCount,
					Fields: defaultFields,
				},
			},
			expectedErr: errInvalidAuthenticationConfigured,
		},
		{
			name: "Invalid Config, no auth credentials",
			config: Config{
				Zone:         "023e105f4ecef8ad9ca31a8372d0c353",
				PollInterval: defaultPollInterval,
				Auth:         &Auth{},
				Logs: &LogsConfig{
					Sample: float32(defaultSampleRate),
					Count:  defaultCount,
					Fields: defaultFields,
				},
			},
			expectedErr: errInvalidAuthenticationConfigured,
		},
		{
			name: "Invalid Config with bad sample rate",
			config: Config{
				PollInterval: defaultPollInterval,
				Zone:         "023e105f4ecef8ad9ca31a8372d0c353",
				Auth: &Auth{
					XAuthKey:   "abc123",
					XAuthEmail: "email@email.com",
				},
				Logs: &LogsConfig{
					Sample: 1.2,
					Count:  defaultCount,
					Fields: defaultFields,
				},
			},
			expectedErr: errInvalidSampleConfigured,
		},
		{
			name: "invalid Config with bad count",
			config: Config{
				PollInterval: defaultPollInterval,
				Zone:         "023e105f4ecef8ad9ca31a8372d0c353",
				Auth: &Auth{
					XAuthKey:   "abc123",
					XAuthEmail: "email@email.com",
				},
				Logs: &LogsConfig{
					Sample: float32(defaultSampleRate),
					Count:  -1,
					Fields: defaultFields,
				},
			},
			expectedErr: errInvalidCountConfigured,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.Validate()
			if tc.expectedErr != nil {
				require.ErrorContains(t, err, tc.expectedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	cases := []struct {
		name           string
		expectedConfig component.Config
	}{
		{
			name: "default",
			expectedConfig: &Config{
				Zone:         "023e105f4ecef8ad9ca31a8372d0c353",
				PollInterval: defaultPollInterval,
				Auth: &Auth{
					XAuthEmail: "email@email.com",
					XAuthKey:   "abc123",
				},
				Logs: &LogsConfig{
					Count:  defaultCount,
					Sample: float32(defaultSampleRate),
					Fields: defaultFields,
				},
			},
		},
		{
			name: "token",
			expectedConfig: &Config{
				Zone:         "023e105f4ecef8ad9ca31a8372d0c353",
				PollInterval: defaultPollInterval,
				Auth: &Auth{
					APIToken: "abc123token",
				},
				Logs: &LogsConfig{
					Count:  defaultCount,
					Sample: float32(defaultSampleRate),
					Fields: defaultFields,
				},
			},
		},
		{
			name: "custom",
			expectedConfig: &Config{
				Zone:         "2",
				PollInterval: 2 * time.Minute,
				Auth: &Auth{
					XAuthEmail: "otel@email.com",
					XAuthKey:   "abc123key",
				},
				Logs: &LogsConfig{
					Count:  50,
					Sample: .5,
					Fields: []string{
						"ClientIP",
						"ClientRequestHost",
						"ClientRequestMethod",
						"ClientRequestURI",
						"EdgeEndTimestamp",
						"EdgeResponseBytes",
						"EdgeResponseStatus",
						"EdgeStartTimestamp",
						"RayID",
					},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			loaded, err := cm.Sub(component.NewIDWithName(typeStr, tc.name).String())
			require.NoError(t, err)
			require.NoError(t, component.UnmarshalConfig(loaded, cfg))
			require.Equal(t, cfg, tc.expectedConfig)
			require.NoError(t, component.ValidateConfig(cfg))
		})
	}
}
