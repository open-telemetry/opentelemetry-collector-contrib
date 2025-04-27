// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs

import (
	"errors"

	"github.com/spf13/pflag"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/internal/common"
)

// Config describes the test scenario.
type Config struct {
	common.Config
	NumLogs        int
	Body           string
	SeverityText   string
	SeverityNumber int32
	TraceID        string
	SpanID         string
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

	fs.IntVar(&c.NumLogs, "logs", c.NumLogs, "Number of logs to generate in each worker (ignored if duration is provided)")
	fs.StringVar(&c.Body, "body", c.Body, "Body of the log")
	fs.StringVar(&c.SeverityText, "severity-text", c.SeverityText, "Severity text of the log")
	fs.Int32Var(&c.SeverityNumber, "severity-number", c.SeverityNumber, "Severity number of the log, range from 1 to 24 (inclusive)")
	fs.StringVar(&c.TraceID, "trace-id", c.TraceID, "TraceID of the log")
	fs.StringVar(&c.SpanID, "span-id", c.SpanID, "SpanID of the log")
}

// SetDefaults sets the default values for the configuration
// This is called before parsing the command line flags and when
// calling NewConfig()
func (c *Config) SetDefaults() {
	c.Config.SetDefaults()
	c.HTTPPath = "/v1/logs"
	c.NumLogs = 1
	c.Body = "the message"
	c.SeverityText = "Info"
	c.SeverityNumber = 9
	c.TraceID = ""
	c.SpanID = ""
}

// Validate validates the test scenario parameters.
func (c *Config) Validate() error {
	if c.TotalDuration <= 0 && c.NumLogs <= 0 {
		return errors.New("either `logs` or `duration` must be greater than 0")
	}

	if c.TraceID != "" {
		if err := common.ValidateTraceID(c.TraceID); err != nil {
			return err
		}
	}

	if c.SpanID != "" {
		if err := common.ValidateSpanID(c.SpanID); err != nil {
			return err
		}
	}

	return nil
}
