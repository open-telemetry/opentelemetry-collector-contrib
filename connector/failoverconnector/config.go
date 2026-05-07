// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package failoverconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector"

import (
	"errors"
	"slices"
	"time"

	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pipeline"
)

var (
	errNoPipelinePriority    = errors.New("No pipelines are defined in the priority list")
	errInvalidRetryIntervals = errors.New("Retry interval must be positive")
	errInvalidStrategy       = errors.New("strategy is invalid")
)

var validStrategies = []Strategy{StrategyStandard}

const defaultRetryInterval = 10 * time.Minute

func durationPtr(d time.Duration) *time.Duration { return &d }

type Config struct {
	// QueueSettings use the exporterhelper sending_queue to move the queue to the connector to avoid data being stuck
	// in the queue of an unhealthy exporter
	QueueSettings configoptional.Optional[exporterhelper.QueueBatchConfig] `mapstructure:"sending_queue"`

	// PipelinePriority is the list of pipeline level priorities in a 1 - n configuration, multiple pipelines can
	// sit at a single priority level and will be routed in a fanout. If any pipeline at a level fails, the
	// level is considered unhealthy
	PipelinePriority [][]pipeline.ID `mapstructure:"priority_levels"`

	// Strategy determines the failover strategy to use. Options: "standard".
	Strategy Strategy `mapstructure:"strategy"`

	// Standard holds configuration that applies when Strategy is "standard".
	Standard StandardConfig `mapstructure:"standard"`

	// RetryInterval is the frequency at which the pipeline levels will attempt to recover by going over
	// all levels below the current
	RetryInterval *time.Duration `mapstructure:"retry_interval"` // **Deprecated**

	// RetryGap is how much time will pass between trying two separate priority levels in a single RetryInterval
	// If the priority list has 3 levels, the RetryInterval is 5m, and the retryGap is 1m, within the 5m RetryInterval,
	// the connector will only try one level every 1m, and will return to the stable level in the interim
	RetryGap time.Duration `mapstructure:"retry_gap"` // **Deprecated**

	// MaxRetry is the maximum retries per level, once this limit is hit for a level, even if the next pipeline level fails,
	// it will not try to recover the level that exceeded the maximum retries
	MaxRetries int `mapstructure:"max_retries"` // **Deprecated**
	// prevent unkeyed literal initialization
	_ struct{}
}

type StandardConfig struct {
	RetryInterval *time.Duration `mapstructure:"retry_interval"`
}

// effectiveRetryInterval returns the standard.retry_interval if set, otherwise
// the deprecated top-level retry_interval, otherwise defaultRetryInterval.
//
// TODO(strategy-config): the top-level retry_interval fallback is temporary.
// When that field is removed in a future release, this resolver collapses to a
// direct read of c.Standard.RetryInterval (with defaultRetryInterval as the
// only fallback).
func (c *Config) effectiveRetryInterval() time.Duration {
	switch {
	case c.Standard.RetryInterval != nil:
		return *c.Standard.RetryInterval
	case c.RetryInterval != nil:
		return *c.RetryInterval
	default:
		return defaultRetryInterval
	}
}

// Validate needs to ensure RetryInterval > # elements in PriorityList * RetryGap
func (c *Config) Validate() error {
	if len(c.PipelinePriority) == 0 {
		return errNoPipelinePriority
	}
	if c.RetryInterval != nil && *c.RetryInterval <= 0 {
		return errInvalidRetryIntervals
	}
	if c.Standard.RetryInterval != nil && *c.Standard.RetryInterval <= 0 {
		return errInvalidRetryIntervals
	}
	if !slices.Contains(validStrategies, c.Strategy) {
		return errInvalidStrategy
	}
	return nil
}
