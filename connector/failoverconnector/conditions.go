// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package failoverconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector"

import (
	"errors"
	"strings"
)

var (
	errNoConditionDefined           = errors.New("no condition is defined")
	errTooManyConditions            = errors.New("only one failover condition can be applied")
	errEmptyErrorContains           = errors.New("error condition must define a non-empty 'contains' string")
	_                     Condition = (*ErrorCondition)(nil)
)

// It plugs in all the available implementations of
// `Condition`. At most only one condition can be set
type ConditionsConfig struct {
	ErrorCond *ErrorCondition `mapstructure:"error"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// Validate ensures exactly one condition is set and that the
// condition itself is valid.
func (c *ConditionsConfig) Validate() error {
	set := 0
	if c.ErrorCond != nil {
		set++
	}
	if set > 1 {
		return errTooManyConditions
	}

	if set == 0 {
		return errNoConditionDefined
	}

	if c.ErrorCond != nil {
		if err := c.ErrorCond.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// All conditions must implement this interface
type Condition interface {
	// ShouldFailover determines if the connector should failover based on current consumer error
	ShouldFailover(err error) bool
}

// ErrorCondition implements Condition
type ErrorCondition struct {
	// Contains is a case-sensitive substring matched against the downstream
	// error message. Only errors containing this string trigger failover.
	Contains string `mapstructure:"contains"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// Validate ensures the error condition has a usable match string.
func (c *ErrorCondition) Validate() error {
	if c.Contains == "" {
		return errEmptyErrorContains
	}
	return nil
}

// ShouldFailover reports whether err should trigger failover.
// A nil error never triggers failover, otherwise failover happens only
// when the error message contains the configured substring.
func (c *ErrorCondition) ShouldFailover(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), c.Contains)
}

func buildCondition(c *ConditionsConfig) Condition {
	if c == nil {
		return nil
	}
	if c.ErrorCond != nil {
		return c.ErrorCond
	}
	return nil
}
