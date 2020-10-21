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
	"go.opentelemetry.io/collector/config/confignet"
)

func TestTags(t *testing.T) {
	tc := TagsConfig{
		Hostname: "customhost",
		Env:      "customenv",
		Service:  "customservice",
		Version:  "customversion",
		Tags:     []string{"key1:val1", "key2:val2"},
	}

	assert.ElementsMatch(t,
		[]string{
			"host:customhost",
			"env:customenv",
			"service:customservice",
			"version:customversion",
			"key1:val1",
			"key2:val2",
		},
		tc.GetTags(true), // get host
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

	err := cfg.Sanitize()
	require.NoError(t, err)
	assert.Equal(t, cfg.Metrics.Endpoint, DebugEndpoint)
}

func TestAPIKeyUnset(t *testing.T) {
	cfg := Config{}
	err := cfg.Sanitize()
	assert.Equal(t, err, errUnsetAPIKey)
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
