// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traces

import (
	"errors"
	"time"

	"github.com/spf13/pflag"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/internal/common"
)

// Config describes the test scenario.
type Config struct {
	common.Config
	NumTraces        int
	NumChildSpans    int
	PropagateContext bool
	StatusCode       string
	Batch            bool
	LoadSize         int

	SpanDuration time.Duration
}

func NewConfig() *Config {
	cfg := &Config{}
	cfg.SetDefaults()
	return cfg
}

// Flags registers config flags.
func (c *Config) Flags(fs *pflag.FlagSet) {
	c.CommonFlags(fs)

	fs.StringVar(&c.HTTPPath, "otlp-http-url-path", c.HTTPPath, "Which URL path to write to")

	fs.IntVar(&c.NumTraces, "traces", c.NumTraces, "Number of traces to generate in each worker (ignored if duration is provided)")
	fs.IntVar(&c.NumChildSpans, "child-spans", c.NumChildSpans, "Number of child spans to generate for each trace")
	fs.BoolVar(&c.PropagateContext, "marshal", c.PropagateContext, "Whether to marshal trace context via HTTP headers")
	fs.StringVar(&c.StatusCode, "status-code", c.StatusCode, "Status code to use for the spans, one of (Unset, Error, Ok) or the equivalent integer (0,1,2)")
	fs.BoolVar(&c.Batch, "batch", c.Batch, "Whether to batch traces")
	fs.IntVar(&c.LoadSize, "size", c.LoadSize, "Desired minimum size in MB of string data for each trace generated. This can be used to test traces with large payloads, i.e. when testing the OTLP receiver endpoint max receive size.")
	fs.DurationVar(&c.SpanDuration, "span-duration", c.SpanDuration, "The duration of each generated span.")
}

// SetDefaults sets the default values for the configuration
// This is called before parsing the command line flags and when
// calling NewConfig()
func (c *Config) SetDefaults() {
	c.Config.SetDefaults()
	c.HTTPPath = "/v1/traces"
	c.NumTraces = 1
	c.NumChildSpans = 1
	c.PropagateContext = false
	c.StatusCode = "0"
	c.Batch = true
	c.LoadSize = 0
	c.SpanDuration = 123 * time.Microsecond
}

// Validate validates the test scenario parameters.
func (c *Config) Validate() error {
	if c.TotalDuration <= 0 && c.NumTraces <= 0 {
		return errors.New("either `traces` or `duration` must be greater than 0")
	}
	return nil
}
