// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package failoverconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector"

import (
	"errors"
)

var (
	errNoConditionDefined = errors.New("no condition is defined")
	errTooManyConditions  = errors.New("only one failover condition can be applied")
)

// At most only one condition can be set
type ConditionsConfig struct {
	ErrorCond *ErrorCondition `mapstructure:"error"`
}

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

type Condition interface {
	// ShouldFailover determines if the connector should failover based on current consumer error
	ShouldFailover(err error) bool
}

type ErrorCondition struct {
	Contains string `mapstructure:"contains"`
}

// TODO: currently no-op
func (e *ErrorCondition) ShouldFailover(err error) bool {
	if err == nil {
		return false
	}
	return true
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
