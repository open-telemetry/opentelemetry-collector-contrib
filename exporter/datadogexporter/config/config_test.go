// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confignet"
	"go.uber.org/zap"
)

// TestOverrideMetricsURL tests that the metrics URL is overridden
// correctly when set manually.
func TestOverrideMetricsURL(t *testing.T) {

	const DebugEndpoint string = "http://localhost:8080"

	cfg := Config{
		API: APIConfig{Key: "notnull", Site: DefaultSite},
		Metrics: MetricsConfig{
			TCPAddr: confignet.TCPAddr{
				Endpoint: DebugEndpoint,
			},
		},
	}

	err := cfg.Sanitize(zap.NewNop())
	require.NoError(t, err)
	assert.Equal(t, cfg.Metrics.Endpoint, DebugEndpoint)
}

// TestDefaultSite tests that the Site option is set to the
// default value when no value was set prior to running Sanitize
func TestDefaultSite(t *testing.T) {
	cfg := Config{
		API: APIConfig{Key: "notnull"},
	}

	err := cfg.Sanitize(zap.NewNop())
	require.NoError(t, err)
	assert.Equal(t, cfg.API.Site, DefaultSite)
}

func TestDefaultTLSSettings(t *testing.T) {
	cfg := Config{
		API: APIConfig{Key: "notnull"},
	}

	err := cfg.Sanitize(zap.NewNop())
	require.NoError(t, err)
	assert.Equal(t, cfg.LimitedHTTPClientSettings.TLSSetting.InsecureSkipVerify, false)
}

func TestTLSSettings(t *testing.T) {
	cfg := Config{
		API: APIConfig{Key: "notnull"},
		LimitedHTTPClientSettings: LimitedHTTPClientSettings{
			TLSSetting: LimitedTLSClientSettings{
				InsecureSkipVerify: true,
			},
		},
	}

	err := cfg.Sanitize(zap.NewNop())
	require.NoError(t, err)
	assert.Equal(t, cfg.TLSSetting.InsecureSkipVerify, true)
}

func TestAPIKeyUnset(t *testing.T) {
	cfg := Config{}
	err := cfg.Sanitize(zap.NewNop())
	assert.Equal(t, err, errUnsetAPIKey)
}

func TestNoMetadata(t *testing.T) {
	cfg := Config{
		OnlyMetadata: true,
		SendMetadata: false,
	}

	err := cfg.Sanitize(zap.NewNop())
	assert.Equal(t, err, errNoMetadata)
}

func TestInvalidHostname(t *testing.T) {
	cfg := Config{TagsConfig: TagsConfig{Hostname: "invalid_host"}}

	err := cfg.Sanitize(zap.NewNop())
	require.Error(t, err)
}

func TestIgnoreResourcesValidation(t *testing.T) {
	validCfg := Config{Traces: TracesConfig{IgnoreResources: []string{"[123]"}}}
	invalidCfg := Config{Traces: TracesConfig{IgnoreResources: []string{"[123"}}}

	noErr := validCfg.Validate()
	err := invalidCfg.Validate()
	require.NoError(t, noErr)
	require.Error(t, err)
}

func TestSpanNameRemappingsValidation(t *testing.T) {
	validCfg := Config{Traces: TracesConfig{SpanNameRemappings: map[string]string{"old.opentelemetryspan.name": "updated.name"}}}
	invalidCfg := Config{Traces: TracesConfig{SpanNameRemappings: map[string]string{"oldname": ""}}}
	noErr := validCfg.Validate()
	err := invalidCfg.Validate()
	require.NoError(t, noErr)
	require.Error(t, err)
}

func TestUnmarshal(t *testing.T) {
	tests := []struct {
		name      string
		configMap *config.Map
		cfg       Config
		err       string
	}{
		{
			name: "invalid cumulative monotonic mode",
			configMap: config.NewMapFromStringMap(map[string]interface{}{
				"metrics": map[string]interface{}{
					"sums": map[string]interface{}{
						"cumulative_monotonic_mode": "invalid_mode",
					},
				},
			}),
			err: "1 error(s) decoding:\n\n* error decoding 'metrics.sums.cumulative_monotonic_mode': invalid cumulative monotonic sum mode \"invalid_mode\"",
		},
		{
			name: "invalid host metadata hostname source",
			configMap: config.NewMapFromStringMap(map[string]interface{}{
				"host_metadata": map[string]interface{}{
					"hostname_source": "invalid_source",
				},
			}),
			err: "1 error(s) decoding:\n\n* error decoding 'host_metadata.hostname_source': invalid host metadata hostname source \"invalid_source\"",
		},
	}

	for _, testInstance := range tests {
		t.Run(testInstance.name, func(t *testing.T) {
			cfg := oldDefaultConfig()
			err := cfg.Unmarshal(testInstance.configMap)
			if err != nil || testInstance.err != "" {
				assert.EqualError(t, err, testInstance.err)
			} else {
				assert.Equal(t, testInstance.cfg, cfg)
			}
		})
	}
}
