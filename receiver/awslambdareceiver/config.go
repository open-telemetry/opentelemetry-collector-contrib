// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awslambdareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awslambdareceiver"

import (
	"go.opentelemetry.io/collector/component"
)

type Config struct {
	// S3Encoding identifies the encoding of the S3 objects that trigger the Lambda.
	//
	// If this is unspecified, the receiver defaults to parsing logs in CloudWatch Log
	// subscription filter format.
	S3Encoding string `mapstructure:"s3_encoding"`
}

var _ component.Config = (*Config)(nil)

func createDefaultConfig() component.Config {
	return &Config{}
}

func (c *Config) Validate() error {
	return nil
}
