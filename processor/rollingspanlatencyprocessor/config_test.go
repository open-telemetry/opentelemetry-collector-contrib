// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package rollingspanlatencyprocessor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
)

func validConfig() *Config {
	return &Config{
		AttributeKey:          "latency.category",
		ResourceKeyAttributes: []string{"service.name"},
		HalfLife:              2 * time.Hour,
		IdleTimeout:           8 * time.Hour,
		EvictionInterval:      10 * time.Minute,
		SlowThreshold:         3.0,
		VerySlowThreshold:     4.0,
		ChurnWarningRatio:     0.5,
		MinStddev:             time.Millisecond,
		MaxBaselines:          0,
		WarmupCount:           30,
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		modify      func(*Config)
		expectedErr string
	}{
		{
			name:        "valid config",
			modify:      func(_ *Config) {},
			expectedErr: "",
		},
		{
			name:        "empty attribute_key",
			modify:      func(c *Config) { c.AttributeKey = "" },
			expectedErr: "attribute_key must not be empty",
		},
		{
			name:        "empty resource_key_attributes",
			modify:      func(c *Config) { c.ResourceKeyAttributes = nil },
			expectedErr: "resource_key_attributes must not be empty",
		},
		{
			name:        "zero half_life",
			modify:      func(c *Config) { c.HalfLife = 0 },
			expectedErr: "half_life must be greater than 0",
		},
		{
			name:        "zero idle_timeout",
			modify:      func(c *Config) { c.IdleTimeout = 0 },
			expectedErr: "idle_timeout must be greater than 0",
		},
		{
			name:        "zero eviction_interval",
			modify:      func(c *Config) { c.EvictionInterval = 0 },
			expectedErr: "eviction_interval must be greater than 0",
		},
		{
			name:        "zero slow_threshold",
			modify:      func(c *Config) { c.SlowThreshold = 0 },
			expectedErr: "slow_threshold must be greater than 0",
		},
		{
			name:        "very_slow_threshold not greater than slow_threshold",
			modify:      func(c *Config) { c.VerySlowThreshold = c.SlowThreshold },
			expectedErr: "very_slow_threshold must be greater than slow_threshold",
		},
		{
			name:        "zero churn_warning_ratio",
			modify:      func(c *Config) { c.ChurnWarningRatio = 0 },
			expectedErr: "churn_warning_ratio must be greater than 0 and less than or equal to 1",
		},
		{
			name:        "churn_warning_ratio too large",
			modify:      func(c *Config) { c.ChurnWarningRatio = 1.1 },
			expectedErr: "churn_warning_ratio must be greater than 0 and less than or equal to 1",
		},
		{
			name:        "negative min_stddev",
			modify:      func(c *Config) { c.MinStddev = -1 },
			expectedErr: "min_stddev must be greater than or equal to 0",
		},
		{
			name:        "negative max_baselines",
			modify:      func(c *Config) { c.MaxBaselines = -1 },
			expectedErr: "max_baselines must be greater than or equal to 0",
		},
		{
			name:        "zero warmup_count",
			modify:      func(c *Config) { c.WarmupCount = 0 },
			expectedErr: "warmup_count must be greater than 0",
		},
		{
			name:        "unrecognized span_kinds value",
			modify:      func(c *Config) { c.SpanKinds = []string{"SERVER", "bogus"} },
			expectedErr: `span_kinds: "bogus" is not a recognized span kind`,
		},
		{
			name:        "span_kinds accepts mixed case",
			modify:      func(c *Config) { c.SpanKinds = []string{"server", "Consumer"} },
			expectedErr: "",
		},
		{
			name:        "always_include_root_spans alone is valid",
			modify:      func(c *Config) { c.AlwaysIncludeRootSpans = true },
			expectedErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.modify(cfg)
			err := cfg.Validate()
			if tt.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			}
		})
	}
}

// TestConfig_UnmarshalFilteringKeys pins the span_kinds / always_include_root_spans
// YAML keys, so a future field rename shows up as a test failure rather than a
// silently ignored (and therefore defaulted) config key.
func TestConfig_UnmarshalFilteringKeys(t *testing.T) {
	cm := confmap.NewFromStringMap(map[string]any{
		"span_kinds":                []any{"SERVER"},
		"always_include_root_spans": true,
	})

	cfg := createDefaultConfig().(*Config)
	require.NoError(t, cm.Unmarshal(cfg))

	assert.Equal(t, []string{"SERVER"}, cfg.SpanKinds)
	assert.True(t, cfg.AlwaysIncludeRootSpans)
}
