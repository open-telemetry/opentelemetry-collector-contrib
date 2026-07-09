// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package failoverconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector"

import (
	"fmt"

	"go.opentelemetry.io/collector/confmap"
)

type ConditionsBuilder func(*confmap.Conf) (Condition, error)

var ConditionsMapping = map[string]ConditionsBuilder{
	"error": NewErrorCondition,
}

type Condition interface {
	// ShouldFailover determines if the connector should failover based on current consumer error
	ShouldFailover(err error) bool
}

type ErrorCondition struct {
	errorContains string `mapstructure:"contains"`
}

func NewErrorCondition(config *confmap.Conf) (Condition, error) {
	e := &ErrorCondition{}
	err := config.Unmarshal(e)
	if err != nil {
		return nil, fmt.Errorf("error building condition `error`: %w", err)
	}

	return e, nil
}

// TODO: currently no-op
func (e *ErrorCondition) ShouldFailover(err error) bool {
	if err == nil {
		return false
	}
	return true
}

type HealthCheckCondition struct {
	Status                   string `mapstructure:"status"`
	healthCheckExtensionHost string `mapstructure:"host"`
	healthCheckExtensionPort string `mapstructure:"host"`
}

func NewHealthCheckCondition(config *confmap.Conf) (Condition, error) {
	e := &HealthCheckCondition{}
	err := config.Unmarshal(e)
	if err != nil {
		return nil, fmt.Errorf("error building condition `error`: %w", err)
	}

	return e, nil
}

// TODO: no-op
func (h *HealthCheckCondition) ShouldFailover(_ error) bool {
	return true
}
