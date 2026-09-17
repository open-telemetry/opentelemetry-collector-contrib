// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package failoverconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector"

import (
	"errors"
)

var (
	errNoConditionDefined           = errors.New("no condition is defined")
	errTooManyConditions            = errors.New("only one failover condition can be applied")
	_                     Condition = (*ErrorCondition)(nil)
)

// It plugs in all the available implementations of
// `Condition`. At most only one condition can be set
type ConditionsConfig struct {
	ErrorCond *ErrorCondition `mapstructure:"error"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// We allow setting `error.contains` but `contains` is not honored yet
// And all errors trigger failover
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

	return nil
}

// All conditions must implement this interface
type Condition interface {
	// ShouldFailover determines if the connector should failover based on current consumer error
	ShouldFailover(err error) bool
}

// ErrorCondition implements Condition
type ErrorCondition struct {
	Contains string `mapstructure:"contains"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// TODO: "contains" condition is not honored yet and all error trigger failover
func (*ErrorCondition) ShouldFailover(err error) bool {
	return err != nil
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
