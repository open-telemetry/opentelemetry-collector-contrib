// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awslambdareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awslambdareceiver"

import (
	"go.opentelemetry.io/collector/component"
)

type Config struct {
	// S3 defines configuration options for S3 Lambda trigger.
	S3 sharedConfig `mapstructure:"s3"`

	// CloudWatch defines configuration options for CloudWatch Lambda trigger.
	CloudWatch sharedConfig `mapstructure:"cloudwatch"`

	_ struct{} // Prevent unkeyed literal initialization
}

// sharedConfig defines configuration options shared between different AWS Lambda trigger types.
// Note - we may extend or split this into dedicated structures per trigger type in future if needed.
type sharedConfig struct {
	// Encoding defines the encoding to decode incoming Lambda invocation data.
	// This extension is expected to further process content of the events that are extracted from Lambda trigger.
	//
	// If receiving data is in different formats(ex:- a mix of VPC flow logs, CloudTrail logs), receiver is recommended
	// to have separate Lambda functions with specific extension configurations.
	Encoding string `mapstructure:"encoding"`
}

var _ component.Config = (*Config)(nil)

func createDefaultConfig() component.Config {
	return &Config{}
}

func (*Config) Validate() error {
	return nil
}
