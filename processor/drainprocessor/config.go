// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package drainprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/drainprocessor"

import (
	"errors"
	"fmt"
)

// Config defines configuration for the drain processor.
type Config struct {
	// LogClusterDepth is the max depth of the Drain parse tree.
	// Higher values produce more specific templates. Default: 4. Minimum: 3.
	LogClusterDepth int `mapstructure:"log_cluster_depth"`

	// SimThreshold is the similarity threshold (0.0–1.0) below which a new
	// cluster is created rather than merged with an existing one. Default: 0.4.
	SimThreshold float64 `mapstructure:"sim_threshold"`

	// MaxChildren is the maximum number of children per parse tree node.
	// Default: 100.
	MaxChildren int `mapstructure:"max_children"`

	// MaxClusters is the maximum number of clusters tracked. When the limit is
	// reached, the least recently used cluster is evicted. 0 means unlimited.
	// Default: 0.
	MaxClusters int `mapstructure:"max_clusters"`

	// ExtraDelimiters are additional token delimiters beyond whitespace.
	ExtraDelimiters []string `mapstructure:"extra_delimiters"`

	// BodyField optionally specifies a top-level key to extract from a
	// structured (map) log body before feeding the value to Drain. If empty,
	// the full body string representation is used. This is a convenience for
	// pipelines where the body is a parsed map (e.g. after json_parser) and
	// the user does not have a move operator to promote the message field back
	// to a plain string body. Pipelines that do have that control should use a
	// move operator instead and leave this unset.
	BodyField string `mapstructure:"body_field"`

	// TemplateAttribute is the log record attribute key to write the derived
	// template string to. Default: "log.record.template".
	TemplateAttribute string `mapstructure:"template_attribute"`

	// SeedTemplates is a list of pre-known template strings to train on at
	// startup before any live logs arrive. Improves template stability across
	// restarts for known log patterns.
	SeedTemplates []string `mapstructure:"seed_templates"`

	// SeedLogs is a list of raw example log lines to train on at startup.
	// Drain derives templates from these lines itself.
	SeedLogs []string `mapstructure:"seed_logs"`

	// WarmupMode controls processor behavior during the initial period before
	// the Drain tree has stabilized. Valid values: "passthrough" (default),
	// "buffer".
	WarmupMode string `mapstructure:"warmup_mode"`

	// WarmupMinClusters is the number of distinct clusters that must be
	// observed before warmup ends. Only used when WarmupMode is "buffer".
	// Default: 10.
	WarmupMinClusters int `mapstructure:"warmup_min_clusters"`

	// WarmupBufferMaxLogs is the maximum number of log records to buffer
	// during warmup before flushing regardless of cluster count. Only used
	// when WarmupMode is "buffer". Must be > 0. Default: 10000.
	WarmupBufferMaxLogs int `mapstructure:"warmup_buffer_max_logs"`
}

const (
	warmupModePassthrough = "passthrough"
	warmupModeBuffer      = "buffer"
)

// Validate checks the Config for invalid values.
func (cfg *Config) Validate() error {
	if cfg.LogClusterDepth < 3 {
		return fmt.Errorf("log_cluster_depth must be >= 3, got %d", cfg.LogClusterDepth)
	}
	if cfg.SimThreshold < 0.0 || cfg.SimThreshold > 1.0 {
		return fmt.Errorf("sim_threshold must be in [0.0, 1.0], got %f", cfg.SimThreshold)
	}
	if cfg.WarmupMode != warmupModePassthrough && cfg.WarmupMode != warmupModeBuffer {
		return fmt.Errorf("warmup_mode must be %q or %q, got %q", warmupModePassthrough, warmupModeBuffer, cfg.WarmupMode)
	}
	if cfg.WarmupMode == warmupModeBuffer && cfg.WarmupMinClusters <= 0 {
		return errors.New("warmup_min_clusters must be > 0 when warmup_mode is \"buffer\"")
	}
	if cfg.WarmupMode == warmupModeBuffer && cfg.WarmupBufferMaxLogs <= 0 {
		return errors.New("warmup_buffer_max_logs must be > 0 when warmup_mode is \"buffer\"")
	}
	return nil
}
