// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package db // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/redactionprocessor/internal/db"

type DBSanitizerConfig struct {
	SQLConfig        SQLConfig        `mapstructure:"sql"`
	RedisConfig      RedisConfig      `mapstructure:"redis"`
	ValkeyConfig     ValkeyConfig     `mapstructure:"valkey"`
	MemcachedConfig  MemcachedConfig  `mapstructure:"memcached"`
	MongoConfig      MongoConfig      `mapstructure:"mongo"`
	OpenSearchConfig OpenSearchConfig `mapstructure:"opensearch"`
	ESConfig         ESConfig         `mapstructure:"es"`
	// AllowFallbackWithoutSystem enables sequential sanitization when `db.system` is missing.
	// This is meant for logs contexts and is set internally, not via YAML.
	AllowFallbackWithoutSystem bool  `mapstructure:"-"`
	SanitizeSpanName           *bool `mapstructure:"sanitize_span_name"`
}

type SQLConfig struct {
	Enabled bool `mapstructure:"enabled"`

	// Attributes specifies which attribute keys to apply SQL sanitization to.
	// If empty, SQL sanitization is applied to all string values in log bodies
	// but not to span/metric attributes
	Attributes []string `mapstructure:"attributes"`
}

type RedisConfig struct {
	Enabled bool `mapstructure:"enabled"`

	// Attributes specifies which attribute keys to apply Redis sanitization to.
	// If empty, Redis sanitization is applied to all string values in log bodies
	// but not to span/metric attributes
	Attributes []string `mapstructure:"attributes"`
}

type ValkeyConfig struct {
	Enabled bool `mapstructure:"enabled"`

	// Attributes specifies which attribute keys to apply Valkey sanitization to.
	// If empty, Valkey sanitization is applied to all string values in log bodies
	// but not to span/metric attributes
	Attributes []string `mapstructure:"attributes"`
}

type MemcachedConfig struct {
	Enabled bool `mapstructure:"enabled"`

	// Attributes specifies which attribute keys to apply Memcached sanitization to.
	// If empty, Memcached sanitization is applied to all string values in log bodies
	// but not to span/metric attributes
	Attributes []string `mapstructure:"attributes"`
}

type MongoConfig struct {
	Enabled bool `mapstructure:"enabled"`

	// Attributes specifies which attribute keys to apply MongoDB sanitization to.
	// If empty, MongoDB sanitization is applied to all string values in log bodies
	// but not to span/metric attributes
	Attributes []string `mapstructure:"attributes"`
}

type OpenSearchConfig struct {
	Enabled bool `mapstructure:"enabled"`

	// Attributes specifies which attribute keys to apply OpenSearch sanitization to.
	// If empty, OpenSearch sanitization is applied to all string values in log bodies
	// but not to span/metric attributes
	Attributes []string `mapstructure:"attributes"`
}

type ESConfig struct {
	Enabled bool `mapstructure:"enabled"`

	// Attributes specifies which attribute keys to apply Elasticsearch sanitization to.
	// If empty, Elasticsearch sanitization is applied to all string values in log bodies
	// but not to span/metric attributes
	Attributes []string `mapstructure:"attributes"`
}
