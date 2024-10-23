// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs

import (
	"fmt"

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

// Flags registers config flags.
func (c *Config) Flags(fs *pflag.FlagSet) {
	c.CommonFlags(fs)

	fs.StringVar(&c.HTTPPath, "otlp-http-url-path", "/v1/logs", "Which URL path to write to")

	fs.IntVar(&c.NumLogs, "logs", 1, "Number of logs to generate in each worker (ignored if duration is provided)")
	fs.StringVar(&c.Body, "body", "the message", "Body of the log")
	fs.StringVar(&c.SeverityText, "severity-text", "Info", "Severity text of the log")
	fs.Int32Var(&c.SeverityNumber, "severity-number", 9, "Severity number of the log, range from 1 to 24 (inclusive)")
	fs.StringVar(&c.TraceID, "trace-id", "", "TraceID of the log")
	fs.StringVar(&c.SpanID, "span-id", "", "SpanID of the log")
}

// Validate validates the test scenario parameters.
func (c *Config) Validate() error {
	if c.TotalDuration <= 0 && c.NumLogs <= 0 {
		return fmt.Errorf("either `logs` or `duration` must be greater than 0")
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
