// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package failoverconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector"

import (
	"fmt"
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
		return fmt.Errorf("only one failover condition can be applied")
	}

	if set == 0 {
		return fmt.Errorf("no conditions are defined")
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

func buildCondition(c ConditionsConfig) Condition {
	if c.ErrorCond != nil {
		return c.ErrorCond
	}
	return nil
}
