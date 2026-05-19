// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package schemaprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor"

import (
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/translation"
)

var (
	errRequiresTargets          = errors.New("requires schema targets")
	errDuplicateTargets         = errors.New("duplicate targets detected")
	errMigrationTargetNotFound  = errors.New("migration target does not match any configured target")
	errMigrationFamilyMismatch  = errors.New("migration from and target must be in the same schema family")
	errMigrationDuplicateTarget = errors.New("duplicate migration entry for same target")
	errMigrationRequiresFrom    = errors.New("migration entry requires a from schema URL")
)

// Config defines the user provided values for the Schema Processor
type Config struct {
	confighttp.ClientConfig `mapstructure:",squash"`

	// CacheCooldown is the duration to wait before retrying schema fetches
	// after the retry limit has been reached. Defaults to 5 minutes.
	CacheCooldown time.Duration `mapstructure:"cache_cooldown"`

	// CacheRetryLimit is the number of consecutive failed schema fetch
	// attempts allowed before enforcing the cooldown period. Defaults to 5.
	CacheRetryLimit int `mapstructure:"cache_retry_limit"`

	// PreCache is a list of schema URLs that are downloaded
	// and cached at the start of the collector runtime
	// in order to avoid fetching data that later on could
	// block processing of signals. (Optional field)
	Prefetch []string `mapstructure:"prefetch"`

	// Targets define what schema families should be
	// translated to, allowing older and newer formats
	// to conform to the target schema identifier.
	Targets []string `mapstructure:"targets"`

	// StorageID is an optional storage extension used to persist fetched
	// schema files across collector restarts. When set, fetched schemas
	// are saved to persistent storage and loaded on startup, avoiding
	// unnecessary HTTP fetches.
	StorageID *component.ID `mapstructure:"storage"`

	// Migration defines migration entries that preserve original attributes
	// alongside renamed ones during schema translation. Each entry specifies
	// a target and the version being migrated from. Only renames between
	// the from version and the target version are copied.
	Migration []MigrationEntry `mapstructure:"migration"`
}

// MigrationEntry defines a migration for a specific target schema.
type MigrationEntry struct {
	// Target is the schema URL that this migration applies to.
	// Must match one of the configured targets exactly.
	Target string `mapstructure:"target"`

	// From is the schema URL of the version that operators are migrating
	// away from. Must be in the same schema family as Target.
	// Renames between this version and the target are copied
	// (both old and new attribute names are preserved).
	From string `mapstructure:"from"`
}

func (c *Config) Validate() error {
	if c.CacheCooldown < 0 {
		return errors.New("cache_cooldown must not be negative")
	}
	if c.CacheRetryLimit < 0 {
		return errors.New("cache_retry_limit must not be negative")
	}
	for _, schemaURL := range c.Prefetch {
		_, _, err := translation.GetFamilyAndVersion(schemaURL)
		if err != nil {
			return err
		}
	}
	// Not strictly needed since it would just pass on
	// any data that doesn't match targets, however defining
	// this processor with no targets is wasteful.
	if len(c.Targets) == 0 {
		return fmt.Errorf("no schema targets defined: %w", errRequiresTargets)
	}

	targets := make(map[string]struct{})
	families := make(map[string]struct{})
	for _, target := range c.Targets {
		family, _, err := translation.GetFamilyAndVersion(target)
		if err != nil {
			return err
		}
		if _, exist := families[family]; exist {
			return errDuplicateTargets
		}
		families[family] = struct{}{}
		targets[target] = struct{}{}
	}

	migrationTargets := make(map[string]struct{})
	for _, entry := range c.Migration {
		if entry.From == "" {
			return errMigrationRequiresFrom
		}
		if _, match := targets[entry.Target]; !match {
			return fmt.Errorf("%q: %w", entry.Target, errMigrationTargetNotFound)
		}
		if _, dup := migrationTargets[entry.Target]; dup {
			return fmt.Errorf("%q: %w", entry.Target, errMigrationDuplicateTarget)
		}
		migrationTargets[entry.Target] = struct{}{}

		targetFamily, _, err := translation.GetFamilyAndVersion(entry.Target)
		if err != nil {
			return fmt.Errorf("migration.target: %w", err)
		}
		fromFamily, _, err := translation.GetFamilyAndVersion(entry.From)
		if err != nil {
			return fmt.Errorf("migration.from: %w", err)
		}
		if targetFamily != fromFamily {
			return fmt.Errorf("migration %q -> %q: %w", entry.From, entry.Target, errMigrationFamilyMismatch)
		}
	}

	return nil
}
