// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package deltatocumulativeprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor"

import (
	"go.opentelemetry.io/collector/component"
)

var _ component.ConfigValidator = (*Config)(nil)

type Config struct{}

func (c *Config) Validate() error {
	return nil
}
