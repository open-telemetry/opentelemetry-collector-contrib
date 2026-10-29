// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package drainprocessor

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/drainprocessor/internal/metadata"
)

func TestConfigValidate(t *testing.T) {
	validCfg := func() *Config {
		return createDefaultConfig().(*Config)
	}

	storageID := component.MustNewID("file_storage")

	tests := []struct {
		name        string
		mutate      func(*Config)
		expectedErr string
	}{
		{
			name:   "default config is valid",
			mutate: func(_ *Config) {},
		},
		{
			name:        "tree_depth below minimum",
			mutate:      func(c *Config) { c.TreeDepth = 2 },
			expectedErr: "tree_depth must be >= 3, got 2",
		},
		{
			name:   "tree_depth at minimum",
			mutate: func(c *Config) { c.TreeDepth = 3 },
		},
		{
			name:        "merge_threshold below range",
			mutate:      func(c *Config) { c.MergeThreshold = -0.1 },
			expectedErr: "merge_threshold must be in [0.0, 1.0], got -0.100000",
		},
		{
			name:        "merge_threshold above range",
			mutate:      func(c *Config) { c.MergeThreshold = 1.1 },
			expectedErr: "merge_threshold must be in [0.0, 1.0], got 1.100000",
		},
		{
			name:   "merge_threshold at lower boundary",
			mutate: func(c *Config) { c.MergeThreshold = 0.0 },
		},
		{
			name:   "merge_threshold at upper boundary",
			mutate: func(c *Config) { c.MergeThreshold = 1.0 },
		},
		{
			name:        "max_node_children zero",
			mutate:      func(c *Config) { c.MaxNodeChildren = 0 },
			expectedErr: "max_node_children must be > 0, got 0",
		},
		{
			name:        "max_node_children negative",
			mutate:      func(c *Config) { c.MaxNodeChildren = -1 },
			expectedErr: "max_node_children must be > 0, got -1",
		},
		{
			name:   "max_node_children positive",
			mutate: func(c *Config) { c.MaxNodeChildren = 50 },
		},
		{
			name:        "max_clusters negative",
			mutate:      func(c *Config) { c.MaxClusters = -1 },
			expectedErr: "max_clusters must be >= 0, got -1",
		},
		{
			name:   "max_clusters zero (unlimited)",
			mutate: func(c *Config) { c.MaxClusters = 0 },
		},
		{
			name:   "max_clusters positive",
			mutate: func(c *Config) { c.MaxClusters = 1000 },
		},
		{
			name:        "template_attribute empty",
			mutate:      func(c *Config) { c.TemplateAttribute = "" },
			expectedErr: "template_attribute must not be empty",
		},
		{
			name:        "warmup_min_clusters negative",
			mutate:      func(c *Config) { c.WarmupMinClusters = -1 },
			expectedErr: "warmup_min_clusters must be >= 0, got -1",
		},
		{
			name:   "warmup_min_clusters zero",
			mutate: func(c *Config) { c.WarmupMinClusters = 0 },
		},
		{
			name:   "warmup_min_clusters positive",
			mutate: func(c *Config) { c.WarmupMinClusters = 20 },
		},
		{
			name:        "save_interval negative",
			mutate:      func(c *Config) { c.SaveInterval = -time.Minute },
			expectedErr: "save_interval must be >= 0, got -1m0s",
		},
		{
			name:        "save_interval positive without storage",
			mutate:      func(c *Config) { c.SaveInterval = 5 * time.Minute },
			expectedErr: "save_interval requires storage to be set",
		},
		{
			name: "save_interval positive with storage",
			mutate: func(c *Config) {
				c.Storage = &storageID
				c.SaveInterval = 5 * time.Minute
			},
		},
		{
			name: "emit_wildcards true with empty wildcards_attribute",
			mutate: func(c *Config) {
				c.EmitWildcards = true
				c.WildcardsAttribute = ""
			},
			expectedErr: "wildcards_attribute must not be empty when emit_wildcards is true",
		},
		{
			name: "masking_rules configured with empty parameter_key_prefix",
			mutate: func(c *Config) {
				c.ParameterKeyPrefix = ""
				c.MaskingRules = []MaskingRule{{Name: "ip", Pattern: `\d+`}}
			},
			expectedErr: "parameter_key_prefix must not be empty when masking_rules are configured",
		},
		{
			name:   "empty masking_rules slice is valid",
			mutate: func(c *Config) { c.MaskingRules = []MaskingRule{} },
		},
		{
			name: "valid masking rule",
			mutate: func(c *Config) {
				c.MaskingRules = []MaskingRule{{Name: "ip", Pattern: `\d+\.\d+\.\d+\.\d+`}}
			},
		},
		{
			name: "masking rule empty name",
			mutate: func(c *Config) {
				c.MaskingRules = []MaskingRule{{Name: "", Pattern: `\d+`}}
			},
			expectedErr: "masking_rules[0]: name must not be empty",
		},
		{
			name: "masking rule name is asterisk",
			mutate: func(c *Config) {
				c.MaskingRules = []MaskingRule{{Name: "*", Pattern: `\d+`}}
			},
			expectedErr: `masking_rules[0]: name must not be "*" (reserved for Drain's wildcard)`,
		},
		{
			name: "masking rule name contains angle bracket",
			mutate: func(c *Config) {
				c.MaskingRules = []MaskingRule{{Name: "i<p", Pattern: `\d+`}}
			},
			expectedErr: `masking_rules[0]: name "i<p" must not contain angle brackets or whitespace`,
		},
		{
			name: "masking rule name contains whitespace",
			mutate: func(c *Config) {
				c.MaskingRules = []MaskingRule{{Name: "ip addr", Pattern: `\d+`}}
			},
			expectedErr: `masking_rules[0]: name "ip addr" must not contain angle brackets or whitespace`,
		},
		{
			name: "masking rule empty pattern",
			mutate: func(c *Config) {
				c.MaskingRules = []MaskingRule{{Name: "ip", Pattern: ""}}
			},
			expectedErr: "masking_rules[0]: pattern must not be empty",
		},
		{
			name: "masking rule invalid regex",
			mutate: func(c *Config) {
				c.MaskingRules = []MaskingRule{{Name: "ip", Pattern: `(`}}
			},
			expectedErr: "masking_rules[0]: pattern \"(\" is not a valid regexp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validCfg()
			tt.mutate(cfg)
			err := cfg.Validate()
			if tt.expectedErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	storageID := component.MustNewID("file_storage")

	tests := []struct {
		id          component.ID
		expected    component.Config
		expectedErr string
	}{
		{
			id:       component.NewIDWithName(metadata.Type, ""),
			expected: createDefaultConfig(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "custom"),
			expected: &Config{
				TreeDepth:          5,
				MergeThreshold:     0.6,
				MaxNodeChildren:    200,
				MaxClusters:        500,
				ExtraDelimiters:    []string{"-", "_"},
				BodyField:          "message",
				TemplateAttribute:  "custom.template",
				ParameterKeyPrefix: "custom.param",
				EmitWildcards:      true,
				WildcardsAttribute: "custom.wildcards",
				SeedTemplates:      []string{"user <*> logged in"},
				SeedLogs:           []string{"user 123 logged in"},
				WarmupMinClusters:  10,
				Storage:            &storageID,
				SaveInterval:       10 * time.Minute,
				MaskingRules: []MaskingRule{
					{
						Name:    "ip",
						Pattern: `\d+\.\d+\.\d+\.\d+`,
					},
				},
			},
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_tree_depth"),
			expectedErr: "tree_depth must be >= 3, got 2",
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_merge_threshold_low"),
			expectedErr: "merge_threshold must be in [0.0, 1.0], got -0.100000",
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_merge_threshold_high"),
			expectedErr: "merge_threshold must be in [0.0, 1.0], got 1.500000",
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_max_node_children"),
			expectedErr: "max_node_children must be > 0, got 0",
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_max_clusters"),
			expectedErr: "max_clusters must be >= 0, got -1",
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_template_attribute"),
			expectedErr: "template_attribute must not be empty",
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_warmup_min_clusters"),
			expectedErr: "warmup_min_clusters must be >= 0, got -1",
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_save_interval_negative"),
			expectedErr: "save_interval must be >= 0, got -5m0s",
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_save_interval_without_storage"),
			expectedErr: "save_interval requires storage to be set",
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_emit_wildcards"),
			expectedErr: "wildcards_attribute must not be empty when emit_wildcards is true",
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_parameter_key_prefix"),
			expectedErr: "parameter_key_prefix must not be empty when masking_rules are configured",
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_masking_rule"),
			expectedErr: "masking_rules[0]: name \"invalid name\" must not contain angle brackets or whitespace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			err = confmap.Validate(cfg)
			if tt.expectedErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, cfg)
			}
		})
	}
}
