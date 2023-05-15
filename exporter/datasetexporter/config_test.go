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
	"testing"

	"github.com/stretchr/testify/suite"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

type SuiteConfig struct {
	suite.Suite
}

func TestSuiteConfig(t *testing.T) {
	suite.Run(t, new(SuiteConfig))
}

func (s *SuiteConfig) TestConfigUnmarshalUnknownAttributes() {
	config := Config{}
	conf := confmap.NewFromStringMap(map[string]interface{}{
		"dataset_url":       "https://example.com",
		"api_key":           "secret",
		"unknown_attribute": "some value",
	})

	err := config.Unmarshal(conf)
	s.NotNil(err)

	unmarshalErr := fmt.Errorf("1 error(s) decoding:\n\n* '' has invalid keys: unknown_attribute")
	expectedError := fmt.Errorf("cannot unmarshal config: %w", unmarshalErr)

	s.Equal(expectedError.Error(), err.Error())
}

func (s *SuiteConfig) TestConfigKeepValuesWhenEnvSet() {
	s.T().Setenv("DATASET_URL", "https://example.org")
	s.T().Setenv("DATASET_API_KEY", "api_key")

	config := Config{}
	conf := confmap.NewFromStringMap(map[string]interface{}{
		"dataset_url": "https://example.com",
		"api_key":     "secret",
	})
	err := config.Unmarshal(conf)
	s.Nil(err)

	s.Equal("https://example.com", config.DatasetURL)
	s.Equal("secret", config.APIKey)
}

func (s *SuiteConfig) TestConfigUseEnvWhenSet() {
	s.T().Setenv("DATASET_URL", "https://example.org")
	s.T().Setenv("DATASET_API_KEY", "api_key")

	config := Config{}
	conf := confmap.NewFromStringMap(map[string]interface{}{})
	err := config.Unmarshal(conf)
	s.Nil(err)

	s.Equal("https://example.org", config.DatasetURL)
	s.Equal("api_key", config.APIKey)
}

func (s *SuiteConfig) TestConfigUseDefaultForMaxDelay() {
	config := Config{}
	conf := confmap.NewFromStringMap(map[string]interface{}{
		"dataset_url":  "https://example.com",
		"api_key":      "secret",
		"max_delay_ms": "",
	})
	err := config.Unmarshal(conf)
	s.Nil(err)

	s.Equal("https://example.com", config.DatasetURL)
	s.Equal("secret", config.APIKey)
	s.Equal("15000", config.MaxDelayMs)
}

func (s *SuiteConfig) TestConfigValidate() {
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
		s.T().Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if err == nil {
				s.Nil(tt.expected, tt.name)
			} else {
				s.Equal(tt.expected.Error(), err.Error(), tt.name)
			}
		})
	}
}

func (s *SuiteConfig) TestConfigString() {
	config := Config{
		DatasetURL:      "https://example.com",
		APIKey:          "secret",
		MaxDelayMs:      "1234",
		GroupBy:         []string{"field1", "field2"},
		RetrySettings:   exporterhelper.NewDefaultRetrySettings(),
		QueueSettings:   exporterhelper.NewDefaultQueueSettings(),
		TimeoutSettings: exporterhelper.NewDefaultTimeoutSettings(),
	}

	s.Equal(
		"DatasetURL: https://example.com; MaxDelayMs: 1234; GroupBy: [field1 field2]; RetrySettings: {Enabled:true InitialInterval:5s RandomizationFactor:0.5 Multiplier:1.5 MaxInterval:30s MaxElapsedTime:5m0s}; QueueSettings: {Enabled:true NumConsumers:10 QueueSize:1000 StorageID:<nil>}; TimeoutSettings: {Timeout:5s}",
		config.String(),
	)
}
