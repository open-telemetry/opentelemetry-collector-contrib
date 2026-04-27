// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package schemaprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor"

import (
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/config/confighttp"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/translation"
)

var (
	errRequiresTargets  = errors.New("requires schema targets")
	errDuplicateTargets = errors.New("duplicate targets detected")
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
	}

	return nil
}
