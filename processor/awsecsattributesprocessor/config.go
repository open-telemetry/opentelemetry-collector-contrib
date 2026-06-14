// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsecsattributesprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/awsecsattributesprocessor"

import (
	"errors"
	"fmt"
	"regexp"

	"go.opentelemetry.io/collector/component"
)

// Config defines the configuration for the awsecsattributes processor.
type Config struct {
	// Attributes is a list of regex patterns matching the attribute keys to be
	// added to the resource. An empty list collects all available attributes.
	Attributes []string `mapstructure:"attributes"`

	// ContainerID specifies which resource attribute fields to read the
	// container ID from, e.g.:
	//   container_id:
	//     sources:
	//       - "log.file.name"
	//       - "container.id"
	ContainerID `mapstructure:"container_id"`

	// CacheTTL is the time to live, in seconds, for the metadata cache.
	CacheTTL int64 `mapstructure:"cache_ttl"`

	// attrExpressions holds the compiled Attributes patterns. It is populated
	// by init() and used by allowAttr() to filter enrichment attributes.
	attrExpressions []*regexp.Regexp

	// prevent unkeyed literal initialization
	_ struct{}
}

// ContainerID configures where the processor looks for the container ID.
type ContainerID struct {
	Sources []string `mapstructure:"sources"`

	// prevent unkeyed literal initialization
	_ struct{}
}

var _ component.Config = (*Config)(nil)

// Validate validates the configuration.
func (c *Config) Validate() error {
	// at least one container ID source must be specified
	if len(c.Sources) == 0 {
		return errors.New("at least one container ID source must be specified [container_id.sources]")
	}

	// cache ttl cannot be less than 60 seconds
	if c.CacheTTL < 60 {
		return errors.New("cache_ttl cannot be less than 60 seconds")
	}

	// validate attribute regex patterns
	for _, expr := range c.Attributes {
		if _, err := regexp.Compile(expr); err != nil {
			return fmt.Errorf("invalid expression found under attributes pattern %s - %w", expr, err)
		}
	}
	return nil
}

// init validates the config and compiles the attribute regular expressions.
func (c *Config) init() error {
	if err := c.Validate(); err != nil {
		return err
	}

	c.attrExpressions = make([]*regexp.Regexp, 0, len(c.Attributes))
	for _, expr := range c.Attributes {
		r, err := regexp.Compile(expr)
		if err != nil {
			return fmt.Errorf("invalid expression found under attributes pattern %s - %w", expr, err)
		}
		c.attrExpressions = append(c.attrExpressions, r)
	}
	return nil
}

// allowAttr reports whether the attribute key matches any configured pattern.
// When no patterns are configured, all attributes are allowed.
func (c *Config) allowAttr(k string) bool {
	if len(c.attrExpressions) == 0 {
		return true
	}

	for _, re := range c.attrExpressions {
		if re.MatchString(k) {
			return true
		}
	}
	return false
}
