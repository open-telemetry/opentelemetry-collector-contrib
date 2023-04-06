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

package datasetexporter

import (
	"fmt"
	"os"
	"testing"

	"github.com/maxatome/go-testdeep/helpers/tdsuite"
	"github.com/maxatome/go-testdeep/td"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

type SuiteConfig struct{}

func (s *SuiteConfig) PreTest(t *td.T, testName string) error {
	os.Clearenv()
	return nil
}

func (s *SuiteConfig) PostTest(t *td.T, testName string) error {
	os.Clearenv()
	return nil
}

func (s *SuiteConfig) Destroy(t *td.T) error {
	os.Clearenv()
	return nil
}

func TestSuiteConfig(t *testing.T) {
	td.NewT(t)
	tdsuite.Run(t, &SuiteConfig{})
}

func (s *SuiteConfig) TestConfigUnmarshalUnknownAttributes(assert, require *td.T) {
	config := Config{}
	conf := confmap.NewFromStringMap(map[string]interface{}{
		"dataset_url":       "https://example.com",
		"api_key":           "secret",
		"unknown_attribute": "some value",
	})

	err := config.Unmarshal(conf)
	assert.NotNil(err)

	unmarshalErr := fmt.Errorf("1 error(s) decoding:\n\n* '' has invalid keys: unknown_attribute")
	expectedError := fmt.Errorf("cannot unmarshal config: %w", unmarshalErr)

	assert.Cmp(err.Error(), expectedError.Error())
	// TODO: Why it does not work?
	// assert.CmpErrorIs(err, expectedError)
}

func (s *SuiteConfig) TestConfigKeepValuesWhenEnvSet(assert, require *td.T) {
	assert.Setenv("DATASET_URL", "https://example.org")
	assert.Setenv("DATASET_API_KEY", "api_key")

	config := Config{}
	conf := confmap.NewFromStringMap(map[string]interface{}{
		"dataset_url": "https://example.com",
		"api_key":     "secret",
	})
	err := config.Unmarshal(conf)
	assert.Nil(err)

	assert.Cmp(config.DatasetURL, "https://example.com")
	assert.Cmp(config.APIKey, "secret")
}

func (s *SuiteConfig) TestConfigUseEnvWhenSet(assert, require *td.T) {
	assert.Setenv("DATASET_URL", "https://example.org")
	assert.Setenv("DATASET_API_KEY", "api_key")

	config := Config{}
	conf := confmap.NewFromStringMap(map[string]interface{}{})
	err := config.Unmarshal(conf)
	assert.Nil(err)

	assert.Cmp(config.DatasetURL, "https://example.org")
	assert.Cmp(config.APIKey, "api_key")
}

func (s *SuiteConfig) TestConfigUseDefaultForMaxDelay(assert, require *td.T) {
	config := Config{}
	conf := confmap.NewFromStringMap(map[string]interface{}{
		"dataset_url":  "https://example.com",
		"api_key":      "secret",
		"max_delay_ms": "",
	})
	err := config.Unmarshal(conf)
	assert.Nil(err)

	assert.Cmp(config.DatasetURL, "https://example.com")
	assert.Cmp(config.APIKey, "secret")
	assert.Cmp(config.MaxDelayMs, "15000")
}

func (s *SuiteConfig) TestConfigValidate(assert, require *td.T) {
	tests := []struct {
		name     string
		config   Config
		expected error
	}{
		{
			name: "valid config",
			config: Config{
				DatasetURL: "https://example.com",
				APIKey:     "secret",
				MaxDelayMs: "12345",
			},
			expected: nil,
		},
		{
			name: "missing api_key",
			config: Config{
				DatasetURL: "https://example.com",
				MaxDelayMs: "15000",
			},
			expected: fmt.Errorf("api_key is required"),
		},
		{
			name: "missing dataset_url",
			config: Config{
				APIKey:     "1234",
				MaxDelayMs: "15000",
			},
			expected: fmt.Errorf("dataset_url is required"),
		},
		{
			name: "invalid max_delay_ms",
			config: Config{
				DatasetURL: "https://example.com",
				APIKey:     "1234",
				MaxDelayMs: "abc",
			},
			expected: fmt.Errorf("max_delay_ms must be integer, but abc was used: strconv.Atoi: parsing \"abc\": invalid syntax"),
		},
	}

	for _, tt := range tests {
		require.Run(tt.name, func(*td.T) {
			err := tt.config.Validate()
			// TODO: Figure out
			// assert.CmpErrorIs(err, tt.expected)
			if err == nil {
				assert.Nil(tt.expected, tt.name)
			} else {
				assert.Cmp(err.Error(), tt.expected.Error(), tt.name)
			}
		})
	}
}

func (s *SuiteConfig) TestConfigString(assert, require *td.T) {
	config := Config{
		DatasetURL:      "https://example.com",
		APIKey:          "secret",
		MaxDelayMs:      "1234",
		GroupBy:         []string{"field1", "field2"},
		RetrySettings:   exporterhelper.NewDefaultRetrySettings(),
		QueueSettings:   exporterhelper.NewDefaultQueueSettings(),
		TimeoutSettings: exporterhelper.NewDefaultTimeoutSettings(),
	}

	assert.Cmp(
		config.String(),
		"DatasetURL: https://example.com; MaxDelayMs: 1234; GroupBy: [field1 field2]; RetrySettings: {Enabled:true InitialInterval:5s RandomizationFactor:0.5 Multiplier:1.5 MaxInterval:30s MaxElapsedTime:5m0s}; QueueSettings: {Enabled:true NumConsumers:10 QueueSize:5000 StorageID:<nil>}; TimeoutSettings: {Timeout:5s}",
	)
}
