// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awslambdareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awslambdareceiver"

import (
	"go.opentelemetry.io/collector/component"
)

const (
	awsLogsEncoding = "awslogs_encoding"
	s3ARNPrefix     = "arn:aws:s3:::"
)

type Config struct {
	// S3Encoding identifies the encoding of the S3 objects that trigger the Lambda.
	//
	// If S3 data is in multiple formats (ex:- VPC flow logs, CloudTrail logs), you should deploy
	// separate Lambda functions with specific extension configurations.
	//
	// If unspecified, the receiver falls back to work with CloudWatch Log subscription encoding extension.
	S3Encoding string `mapstructure:"s3_encoding"`

	_ struct{} // Prevent unkeyed literal initialization
}

var _ component.Config = (*Config)(nil)

func createDefaultConfig() component.Config {
	return &Config{}
}

func (*Config) Validate() error {
	return nil
}
