// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traces

import (
	"time"

	"github.com/spf13/pflag"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/internal/common"
)

// Config describes the test scenario.
type Config struct {
	common.Config
	NumTraces        int
	PropagateContext bool
	ServiceName      string
	StatusCode       string
	Batch            bool
	LoadSize         int

	SpanDuration time.Duration

	// OTLP TLS configuration 
	CaFile string

	// OTLP mTLS configuration
	ClientAuth struct {
		Enabled        bool
		ClientCertFile string
		ClientKeyFile  string
	}

	// OTLP exporter connection timeout
	TimeOut      time.Duration
}

// Flags registers config flags.
func (c *Config) Flags(fs *pflag.FlagSet) {
	c.CommonFlags(fs)

	fs.StringVar(&c.HTTPPath, "otlp-http-url-path", "/v1/traces", "Which URL path to write to")

	fs.IntVar(&c.NumTraces, "traces", 1, "Number of traces to generate in each worker (ignored if duration is provided)")
	fs.BoolVar(&c.PropagateContext, "marshal", false, "Whether to marshal trace context via HTTP headers")
	fs.StringVar(&c.ServiceName, "service", "telemetrygen", "Service name to use")
	fs.StringVar(&c.StatusCode, "status-code", "0", "Status code to use for the spans, one of (Unset, Error, Ok) or the equivalent integer (0,1,2)")
	fs.BoolVar(&c.Batch, "batch", true, "Whether to batch traces")
	fs.IntVar(&c.LoadSize, "size", 0, "Desired minimum size in MB of string data for each trace generated. This can be used to test traces with large payloads, i.e. when testing the OTLP receiver endpoint max receive size.")
	fs.DurationVar(&c.SpanDuration, "span-duration", 123*time.Microsecond, "The duration of each generated span.")

	// TLS CA configuration
	fs.StringVar(&c.CaFile, "ca-cert", "", "Trusted Certificate Authority to verify collector receiver certificate")

	// mTLS configuration
	fs.BoolVar(&c.ClientAuth.Enabled, "mtls", false, "Whether to require client authentication for mTLS")
	fs.StringVar(&c.ClientAuth.ClientCertFile, "client-cert", "", "Client certificate file")
	fs.StringVar(&c.ClientAuth.ClientKeyFile, "client-key", "", "Client private key file")

	// Timeout for connection failures
	fs.DurationVar(&c.TimeOut, "timeout", 60*time.Second, "Timeout setting when testing failure conditions")
}
