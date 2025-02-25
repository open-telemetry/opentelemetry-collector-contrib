// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redactionprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/redactionprocessor"

import "fmt"

type HashFunction string

const (
	NONE HashFunction = ""
	SHA1 HashFunction = "sha1"
	SHA3 HashFunction = "sha3"
	MD5  HashFunction = "md5"
)

type Config struct {
	// AllowAllKeys is a flag to allow all span attribute keys. Setting this
	// to true disables the AllowedKeys list. The list of BlockedValues is
	// applied regardless. If you just want to block values, set this to true.
	AllowAllKeys bool `mapstructure:"allow_all_keys"`

	// AllowedKeys is a list of allowed span attribute keys. Span attributes
	// not on the list are removed. The list fails closed if it's empty. To
	// allow all keys, you should explicitly set AllowAllKeys
	AllowedKeys []string `mapstructure:"allowed_keys"`

	// BlockedKeyPatterns is a list of blocked span attribute key patters. Span attributes
	// matching the regexes on the list are masked.
	BlockedKeyPatterns []string `mapstructure:"blocked_key_patterns"`

	// HashFunction defines the function for hashing the values instead of
	// masking them with a fixed string. By default, no hash function is used
	// and masking with a fixed string is performed.
	HashFunction HashFunction `mapstructure:"hash_function"`

	// IgnoredKeys is a list of span attribute keys that are not redacted.
	// Span attributes in this list are allowed to pass through the filter
	// without being changed or removed.
	IgnoredKeys []string `mapstructure:"ignored_keys"`

	// BlockedValues is a list of regular expressions for blocking values of
	// allowed span attributes. Values that match are masked.
	BlockedValues []string `mapstructure:"blocked_values"`

	// AllowedValues is a list of regular expressions for allowing values of
	// blocked span attributes. Values that match are not masked.
	AllowedValues []string `mapstructure:"allowed_values"`

	// Summary controls the verbosity level of the diagnostic attributes that
	// the processor adds to the spans when it redacts or masks other
	// attributes. In some contexts a list of redacted attributes leaks
	// information, while it is valuable when integrating and testing a new
	// configuration. Possible values are `debug`, `info`, and `silent`.
	Summary string `mapstructure:"summary"`
}

func (cfg *Config) Validate() error {
	return cfg.validateHashFunction()
}

func (cfg *Config) validateHashFunction() error {
	switch cfg.HashFunction {
	case SHA1, SHA3, MD5, NONE:
		return nil
	default:
		return fmt.Errorf("unsupported hash function: '%s'. Supported functions are: 'md5', 'sha1', 'sha3'", cfg.HashFunction)
	}
}
