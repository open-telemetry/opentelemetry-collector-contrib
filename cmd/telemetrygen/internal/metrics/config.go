// Copyright The OpenTelemetry Authors
// Copyright (c) 2018 The Jaeger Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metrics

import (
	"time"

	"github.com/spf13/pflag"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/internal/common"
)

// Config describes the test scenario.
type Config struct {
	common.Config
	NumMetrics int
}

// Flags registers config flags.
func (c *Config) Flags(fs *pflag.FlagSet) {
	fs.IntVar(&c.WorkerCount, "workers", 1, "Number of workers (goroutines) to run")
	fs.IntVar(&c.NumMetrics, "metrics", 1, "Number of metrics to generate in each worker (ignored if duration is provided")
	fs.Int64Var(&c.Rate, "rate", 0, "Approximately how many metrics per second each worker should generate. Zero means no throttling.")
	fs.DurationVar(&c.TotalDuration, "duration", 0, "For how long to run the test")
	fs.DurationVar(&c.ReportingInterval, "interval", 1*time.Second, "Reporting interval (default 1 second)")

	fs.StringVar(&c.Endpoint, "otlp-endpoint", "localhost:4317", "Target to which the exporter is going to send metrics. This MAY be configured to include a path (e.g. example.com/v1/metrics)")
	fs.BoolVar(&c.Insecure, "otlp-insecure", false, "Whether to enable client transport security for the exporter's grpc or http connection")
	fs.BoolVar(&c.UseHTTP, "otlp-http", false, "Whether to use HTTP exporter rather than a gRPC one")

	// custom headers
	c.Headers = make(map[string]string)
	fs.Var(&c.Headers, "otlp-header", "Custom header to be passed along with each OTLP request. The value is expected in the format key=value."+
		"Flag may be repeated to set multiple headers (e.g -otlp-header key1=value1 -otlp-header key2=value2)")

	// custom resource attributes
	c.ResourceAttributes = make(map[string]string)
	fs.Var(&c.ResourceAttributes, "otlp-attributes", "Custom resource attributes to use. The value is expected in the format key=\"value\"."+
		"Flag may be repeated to set multiple attributes (e.g -otlp-attributes key1=\"value1\" -otlp-attributes key2=\"value2\")")
}
