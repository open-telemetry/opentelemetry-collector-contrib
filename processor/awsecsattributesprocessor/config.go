package awsecsattributesprocessor

import (
	"errors"
	"fmt"
	"regexp"
)

type Config struct {
	// Attributes is a list of attribute pattern to be added to the resource
	Attributes []string `mapstructure:"attributes"`

	// Source fields specifies what resource attribute field to read the
	// container ID
	// container_id:
	//   sources:
	//     - "log.file.name"
	// 	   - "container.id"
	ContainerID `mapstructure:"container_id"`

	// CacheTTL is the time to live for the metadata cache
	CacheTTL        int64            `mapstructure:"cache_ttl"`
	attrExpressions []*regexp.Regexp // store compiled regexes
}

type ContainerID struct {
	Sources []string `mapstructure:"sources"`
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// check ContainerID sources
	if len(c.ContainerID.Sources) == 0 {
		return errors.New("atleast one container ID source must be specified. [container_id.sources]")
	}

	// cache ttl cannot be less than 60 seconds
	if c.CacheTTL < 60 {
		return errors.New("cache_ttl cannot be less than 60 seconds")
	}

	// validate attribute regex
	for _, expr := range c.Attributes {
		_, err := regexp.Compile(expr)
		if err != nil {
			return fmt.Errorf("invalid expression found under attributes pattern %s - %w", expr, err)
		}
	}
	return nil
}

// init - initialise config, returns error
func (c *Config) init() error {
	// validate config first
	if err := c.Validate(); err != nil {
		return err
	}

	// compile attribute regex expressions
	for _, expr := range c.Attributes {
		r, err := regexp.Compile(expr)
		if err != nil {
			return fmt.Errorf("invalid expression found under attributes pattern %s - %w", expr, err)
		}

		c.attrExpressions = append(c.attrExpressions, r)
	}
	return nil
}

func (c *Config) allowAttr(k string) bool {
	// if no attribue patterns are present, return true always
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
