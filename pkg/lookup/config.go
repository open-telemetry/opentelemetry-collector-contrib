// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lookup // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/lookup"

import "time"

// Config captures shared configuration fragments for lookup sources
type Config struct {
	Cache CacheConfig `mapstructure:"cache"`
	_     struct{}    `mapstructure:"-"`
}

// CacheConfig describes the cache behavior for a lookup source
type CacheConfig struct {
	Enabled bool          `mapstructure:"enabled"`
	Size    int           `mapstructure:"size"`
	TTL     time.Duration `mapstructure:"ttl"`
	_       struct{}      `mapstructure:"-"`
}
