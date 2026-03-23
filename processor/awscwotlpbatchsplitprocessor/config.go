// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscwotlpbatchsplitprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/awscwotlpbatchsplitprocessor"

import "fmt"

const (
	// 1 MB request size limit for the CloudWatch OTLP endpoint.
	// https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch-OTLPEndpoint.html
	defaultMaxRequestByteSize = 1048576

	// log records can be complex, this is
	// the maximum depth of the nested log object to traverse
	// and calculate the size of before we consider stopping the
	// the calculation
	defaultMaxDepth = 10

	// an estimate of the fixed metadata overhead
	// per OTLP log record. Each log includes fields like traceId, spanId, timestamps,
	// severity, flags that are not captured by body/attribute
	// traversal alone. 2000 bytes is intentionally an overestimate, it is better to
	// overestimate and suffer a minor performance hit from extra batching than to
	// underestimate and risk a log being dropped by the CloudWatch OTLP endpoint.
	defaultBaseLogBufferSize = 2000
)

type Config struct {
	MaxRequestByteSize int `mapstructure:"max_request_byte_size"`
}

func (cfg *Config) Validate() error {
	if cfg.MaxRequestByteSize <= 0 {
		return fmt.Errorf("max_request_byte_size must be > 0, got %d", cfg.MaxRequestByteSize)
	}
	return nil
}
