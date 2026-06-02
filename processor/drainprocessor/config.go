// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package drainprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/drainprocessor"

import (
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
)

// Config defines configuration for the drain processor.
type Config struct {
	// TreeDepth is the max depth of the Drain parse tree (called `depth` in the
	// Drain paper). Higher values produce more specific templates. Default: 4. Minimum: 3.
	TreeDepth int `mapstructure:"tree_depth"`

	// MergeThreshold is the minimum token-match ratio (0.0–1.0) required to merge
	// a log line into an existing cluster rather than creating a new one (called
	// `st` in the Drain paper). Default: 0.4.
	MergeThreshold float64 `mapstructure:"merge_threshold"`

	// MaxNodeChildren is the maximum number of children per internal parse tree node
	// (called `maxChild` in the Drain paper). Bounds memory on high-cardinality
	// token positions. Default: 100.
	MaxNodeChildren int `mapstructure:"max_node_children"`

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

	// WarmupMinClusters is the number of distinct clusters that must be observed
	// before annotation is enabled. During warmup, records pass through immediately
	// but the template attribute is not written — the tree trains on them without
	// emitting unstabilised templates. 0 (default) disables warmup suppression and
	// annotates from the first record.
	WarmupMinClusters int `mapstructure:"warmup_min_clusters"`

	// Storage is the ID of a storage extension to use for persisting the Drain
	// tree across restarts. When set, the tree is loaded on startup and saved on
	// shutdown (and optionally at a periodic interval; see SaveInterval). When a
	// snapshot is loaded successfully, seed_templates and seed_logs are skipped.
	// With a shared storage backend (Redis, database), periodic saves let new
	// instances in a scaled deployment inherit a trained tree from existing
	// instances. Optional — when unset the processor is stateless.
	Storage *component.ID `mapstructure:"storage"`

	// SaveInterval is the interval between periodic snapshot saves to storage.
	// 0 (default) disables periodic saves — the tree is only saved on shutdown.
	// Requires storage to be set.
	SaveInterval time.Duration `mapstructure:"save_interval"`
}

// Validate checks the Config for invalid values.
func (cfg *Config) Validate() error {
	if cfg.TreeDepth < 3 {
		return fmt.Errorf("tree_depth must be >= 3, got %d", cfg.TreeDepth)
	}
	if cfg.MergeThreshold < 0.0 || cfg.MergeThreshold > 1.0 {
		return fmt.Errorf("merge_threshold must be in [0.0, 1.0], got %f", cfg.MergeThreshold)
	}
	if cfg.WarmupMinClusters < 0 {
		return fmt.Errorf("warmup_min_clusters must be >= 0, got %d", cfg.WarmupMinClusters)
	}
	if cfg.SaveInterval < 0 {
		return fmt.Errorf("save_interval must be >= 0, got %s", cfg.SaveInterval)
	}
	if cfg.SaveInterval > 0 && cfg.Storage == nil {
		return errors.New("save_interval requires storage to be set")
	}
	return nil
}
