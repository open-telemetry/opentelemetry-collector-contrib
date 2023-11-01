// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package failoverconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector"

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
)

var (
	errNoPipelinePriority    = errors.New("No pipelines are defined in the priority list")
	errInvalidRetryIntervals = errors.New("Retry interval must be positive, and retry_interval must be greater than retry_gap times the length of the priority list")
)

type Config struct {
	// PipelinePriority is the list of pipeline level priorities in a 1 - n configuration, multiple pipelines can
	// sit at a single priority level and will be routed in a fanout. If any pipeline at a level fails, the
	// level is considered unhealthy
	PipelinePriority [][]component.ID `mapstructure:"priority_levels"`

	// RetryInterval is the frequency at which the pipeline levels will attempt to recover by going over
	// all levels below the current
	RetryInterval time.Duration `mapstructure:"retry_interval"`

	// RetryGap is how much time will pass between trying two separate priority levels in a single RetryInterval
	// If the priority list has 3 levels, the RetryInterval is 5m, and the retryGap is 1m, within the 5m RetryInterval,
	// the connector will only try one level every 1m, and will return to the stable level in the interim
	RetryGap time.Duration `mapstructure:"retry_gap"`

	// MaxRetry is the maximum retries per level, once this limit is hit for a level, even if the next pipeline level fails,
	// it will not try to recover the level that exceeded the maximum retries
	MaxRetries int `mapstructure:"max_retries"`
}

// Validate needs to ensure RetryInterval > # elements in PriorityList * RetryGap
func (c *Config) Validate() error {
	if len(c.PipelinePriority) == 0 {
		return errNoPipelinePriority
	}
	retryTime := c.RetryGap * time.Duration(len(c.PipelinePriority))
	if c.RetryGap <= 0 || c.RetryInterval <= 0 || c.RetryInterval <= retryTime {
		return errInvalidRetryIntervals
	}
	return nil
}
