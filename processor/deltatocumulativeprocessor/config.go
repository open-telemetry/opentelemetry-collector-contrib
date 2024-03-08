// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package deltatocumulativeprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor"

import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
)

var _ component.ConfigValidator = (*Config)(nil)

type Config struct {
	MaxStale time.Duration `json:"max_stale"`
}

func (c *Config) Validate() error {
	if c.MaxStale <= 0 {
		return fmt.Errorf("max_stale must be a positive duration (got %s)", c.MaxStale)
	}
	return nil
}
