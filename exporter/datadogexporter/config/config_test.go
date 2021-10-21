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
	colconfig "go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confignet"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestHostTags(t *testing.T) {
	tc := TagsConfig{
		Hostname: "customhost",
		Env:      "customenv",
		// Service and version should be only used for traces
		Service: "customservice",
		Version: "customversion",
		Tags:    []string{"key1:val1", "key2:val2"},
	}

	assert.ElementsMatch(t,
		[]string{
			"env:customenv",
			"key1:val1",
			"key2:val2",
		},
		tc.GetHostTags(),
	)

	tc = TagsConfig{
		Hostname: "customhost",
		Env:      "customenv",
		// Service and version should be only used for traces
		Service:    "customservice",
		Version:    "customversion",
		Tags:       []string{"key1:val1", "key2:val2"},
		EnvVarTags: "key3:val3 key4:val4",
	}

	assert.ElementsMatch(t,
		[]string{
			"env:customenv",
			"key1:val1",
			"key2:val2",
		},
		tc.GetHostTags(),
	)

	tc = TagsConfig{
		Hostname: "customhost",
		Env:      "customenv",
		// Service and version should be only used for traces
		Service:    "customservice",
		Version:    "customversion",
		EnvVarTags: "key3:val3 key4:val4",
	}

	assert.ElementsMatch(t,
		[]string{
			"env:customenv",
			"key3:val3",
			"key4:val4",
		},
		tc.GetHostTags(),
	)
}

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

func TestCensorAPIKey(t *testing.T) {
	cfg := APIConfig{
		Key: "ddog_32_characters_long_api_key1",
	}

	assert.Equal(
		t,
		"***************************_key1",
		cfg.GetCensoredKey(),
	)
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

func TestDeprecationReportBuckets(t *testing.T) {

	tests := []struct {
		name             string
		stringMap        map[string]interface{}
		expectedMode     string
		expectedWarnings int
	}{
		{
			name: "report buckets false",
			stringMap: map[string]interface{}{
				"api": map[string]interface{}{"key": "aaa"},
				"metrics": map[string]interface{}{
					"report_buckets": false,
				},
			},
			expectedMode:     histogramModeNoBuckets,
			expectedWarnings: 1,
		},
		{
			name: "report buckets true",
			stringMap: map[string]interface{}{
				"api": map[string]interface{}{"key": "aaa"},
				"metrics": map[string]interface{}{
					"report_buckets": true,
				},
			},
			expectedMode:     histogramModeCounters,
			expectedWarnings: 1,
		},
		{
			name: "no report buckets",
			stringMap: map[string]interface{}{
				"api": map[string]interface{}{"key": "aaa"},
			},
			expectedMode:     histogramModeNoBuckets,
			expectedWarnings: 0,
		},
	}

	for _, testInstance := range tests {
		t.Run(testInstance.name, func(t *testing.T) {
			// default config for buckets
			config := Config{Metrics: MetricsConfig{Buckets: false, HistConfig: HistogramConfig{Mode: histogramModeNoBuckets}}}
			configMap := colconfig.NewMapFromStringMap(testInstance.stringMap)
			err := config.Unmarshal(configMap)
			require.NoError(t, err)
			assert.Equal(t, config.Metrics.HistConfig.Mode, testInstance.expectedMode)

			core, observed := observer.New(zapcore.WarnLevel)
			testLogger := zap.New(core)
			config.Sanitize(testLogger)
			assert.Equal(t, len(observed.AllUntimed()), testInstance.expectedWarnings)
		})
	}
}

func TestNoBucketsAndHistogram(t *testing.T) {
	stringMap := map[string]interface{}{
		"api": map[string]interface{}{"key": "aaa"},
		"metrics": map[string]interface{}{
			"report_buckets": false,
			"histograms": map[string]interface{}{
				"mode": histogramModeCounters,
			},
		},
	}

	config := Config{Metrics: MetricsConfig{Buckets: false, HistConfig: HistogramConfig{Mode: histogramModeNoBuckets}}}
	configMap := colconfig.NewMapFromStringMap(stringMap)
	err := config.Unmarshal(configMap)

	assert.Error(t, errBuckets, err)
}
