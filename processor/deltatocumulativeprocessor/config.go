// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package deltatocumulativeprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor"

import (
	"fmt"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/delta"
)

var _ component.ConfigValidator = (*Config)(nil)

type Config struct {
	delta.Options
}

func (c *Config) Validate() error {
	if c.MaxStale <= 0 {
		return fmt.Errorf("maxStale must be a positive duration (got %s)", c.MaxStale)
	}
	return nil
}
