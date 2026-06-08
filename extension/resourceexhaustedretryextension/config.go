// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resourceexhaustedretryextension

import "time"

// Config holds configuration for the resourceexhaustedretryextension.
type Config struct {
	RetryDelay time.Duration `mapstructure:"retry_delay"`
	Jitter     time.Duration `mapstructure:"jitter"`
}
