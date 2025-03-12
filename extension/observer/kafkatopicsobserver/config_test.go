// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkatopicsobserver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/kafkatopicsobserver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka/configkafka"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id            component.ID
		expected      component.Config
		expectedError string
	}{
		{
			id:            component.NewID(metadata.Type),
			expected:      NewFactory().CreateDefaultConfig(),
			expectedError: "protocol_version must be specified; topic_regex must be specified",
		},
		{
			id: component.NewIDWithName(metadata.Type, "all_settings"),
			expected: &Config{
				ProtocolVersion:                      "3.7.0",
				Brokers:                              []string{"1.2.3.4:9092", "2.3.4.5:9092"},
				TopicRegex:                           "^topic[0-9]$",
				TopicsSyncInterval:                   100 * time.Millisecond,
				ResolveCanonicalBootstrapServersOnly: false,
				Authentication: configkafka.AuthenticationConfig{
					PlainText: &configkafka.PlainTextConfig{
						Username: "fooUser",
						Password: "fooPassword",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cfg := loadConfig(t, tt.id)
			if tt.expectedError != "" {
				assert.EqualError(t, xconfmap.Validate(cfg), tt.expectedError)
			} else {
				assert.NoError(t, xconfmap.Validate(cfg))
			}
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestValidateConfig(t *testing.T) {
	cfg := &Config{
		Brokers:         []string{},
		ProtocolVersion: "3.7.0",
		TopicRegex:      "^test[0-9]$",
	}
	assert.Equal(t, "brokers list must be specified; topics_sync_interval must be greater than 0", xconfmap.Validate(cfg).Error())

	cfg = &Config{
		Brokers:            []string{"1.2.3.4:9092"},
		ProtocolVersion:    "",
		TopicRegex:         "^topic[0-9]$",
		TopicsSyncInterval: 1 * time.Second,
	}
	assert.Equal(t, "protocol_version must be specified", xconfmap.Validate(cfg).Error())

	cfg = &Config{
		Brokers:            []string{"1.2.3.4:9092"},
		ProtocolVersion:    "3.7.0",
		TopicRegex:         "",
		TopicsSyncInterval: 1 * time.Second,
	}
	assert.Equal(t, "topic_regex must be specified", xconfmap.Validate(cfg).Error())

	cfg = &Config{
		Brokers:            []string{"1.2.3.4:9092"},
		ProtocolVersion:    "3.7.0",
		TopicRegex:         "^topic[0-9]$",
		TopicsSyncInterval: 0 * time.Second,
	}
	assert.Equal(t, "topics_sync_interval must be greater than 0", xconfmap.Validate(cfg).Error())

	cfg = &Config{
		Brokers:            []string{"1.2.3.4:9092"},
		ProtocolVersion:    "3.7.0",
		TopicRegex:         "^topic[0-9]$",
		TopicsSyncInterval: 1 * time.Second,
	}
	assert.NoError(t, xconfmap.Validate(cfg))
}

func loadConf(tb testing.TB, path string, id component.ID) *confmap.Conf {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", path))
	require.NoError(tb, err)
	sub, err := cm.Sub(id.String())
	require.NoError(tb, err)
	return sub
}

func loadConfig(tb testing.TB, id component.ID) *Config {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	sub := loadConf(tb, "config.yaml", id)
	require.NoError(tb, sub.Unmarshal(cfg))
	return cfg.(*Config)
}
