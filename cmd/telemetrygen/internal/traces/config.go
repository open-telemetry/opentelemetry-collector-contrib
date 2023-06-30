// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traces

import (
	"github.com/spf13/pflag"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/internal/common"
)

// Config describes the test scenario.
type Config struct {
	common.Config
	NumTraces        int
	PropagateContext bool
	ServiceName      string
}

// Flags registers config flags.
func (c *Config) Flags(fs *pflag.FlagSet) {
	c.CommonFlags(fs)
	fs.IntVar(&c.NumTraces, "traces", 1, "Number of traces to generate in each worker (ignored if duration is provided)")
	fs.BoolVar(&c.PropagateContext, "marshal", false, "Whether to marshal trace context via HTTP headers")
	fs.StringVar(&c.ServiceName, "service", "telemetrygen", "Service name to use")
}
