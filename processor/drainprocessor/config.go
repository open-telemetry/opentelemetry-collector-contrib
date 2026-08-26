// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package drainprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/drainprocessor"

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
)

// MaskingRule declares a regex whose matches in the log body are substituted
// with the literal token "<name>" before the body is fed to the Drain tree.
type MaskingRule struct {
	// Name is the mask token used in place of matches. It appears verbatim
	// inside angle brackets in derived templates (e.g. "<ip>") and as the
	// suffix of the corresponding extracted-parameter attribute key
	// ("<ParameterKeyPrefix>.<Name>"). Must be non-empty, must not contain
	// "<", ">", or whitespace, and must not equal "*" (reserved for
	// Drain's own wildcard).
	Name string `mapstructure:"name"`

	// Pattern is a Go regular expression (RE2 syntax). Each match in the
	// working copy of the body is replaced with "<Name>". Capture groups
	// are permitted but ignored — the whole match is replaced.
	Pattern string `mapstructure:"pattern"`
}

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

	// MaskingRules are regex substitutions applied to a working copy of the
	// body before it is fed to the Drain tree. Rules apply in declaration
	// order; each rule runs on the output of the previous rule. Matched
	// substrings become literal mask tokens in derived templates (e.g.
	// "<ip>"), improving tree stability on high-cardinality values. When
	// non-empty, each masked position is written to a dynamic attribute
	// keyed as "<ParameterKeyPrefix>.<name>".
	MaskingRules []MaskingRule `mapstructure:"masking_rules"`

	// ParameterKeyPrefix is the attribute-key prefix for extracted named
	// parameters. Each masked position writes an attribute at
	// "<ParameterKeyPrefix>.<mask name>" with the raw body value. Matches
	// the OpenTelemetry semantic-convention pattern used by
	// http.request.header.<key> and db.query.parameter.<key>. Only
	// consulted when MaskingRules is non-empty. Default:
	// "log.record.template.parameter".
	ParameterKeyPrefix string `mapstructure:"parameter_key_prefix"`

	// EmitWildcards, when true, writes a positional string slice attribute
	// containing the body tokens at each Drain <*> position in template
	// order. Independent of MaskingRules: enable this to see raw variable
	// values without configuring any masks, or in combination with
	// MaskingRules to capture positions no rule named. Default: false.
	EmitWildcards bool `mapstructure:"emit_wildcards"`

	// WildcardsAttribute is the log record attribute key for the wildcards
	// slice. Only consulted when EmitWildcards is true. Default:
	// "log.record.template.wildcards".
	WildcardsAttribute string `mapstructure:"wildcards_attribute"`

	// SeedTemplates is a list of pre-known template strings to train on at
	// startup before any live logs arrive. Improves template stability across
	// restarts for known log patterns.
	SeedTemplates []string `mapstructure:"seed_templates"`

	// SeedLogs is a list of raw example log lines to train on at startup.
	// Drain derives templates from these lines itself. Masking rules apply
	// to seed logs the same way they apply to live records.
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
	for i, r := range cfg.MaskingRules {
		if err := validateMaskingRule(r); err != nil {
			return fmt.Errorf("masking_rules[%d]: %w", i, err)
		}
	}
	return nil
}

func validateMaskingRule(r MaskingRule) error {
	if r.Name == "" {
		return errors.New("name must not be empty")
	}
	if r.Name == "*" {
		return errors.New(`name must not be "*" (reserved for Drain's wildcard)`)
	}
	if strings.ContainsAny(r.Name, "<> \t\n\r") {
		return fmt.Errorf("name %q must not contain angle brackets or whitespace", r.Name)
	}
	if r.Pattern == "" {
		return errors.New("pattern must not be empty")
	}
	if _, err := regexp.Compile(r.Pattern); err != nil {
		return fmt.Errorf("pattern %q is not a valid regexp: %w", r.Pattern, err)
	}
	return nil
}
