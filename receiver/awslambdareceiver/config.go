// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awslambdareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awslambdareceiver"

import (
	"go.opentelemetry.io/collector/component"
)

type Config struct {
	// S3Encoding identifies the encoding of the S3 objects that trigger the Lambda.
	//
	// If S3Encoding is unspecified, the receiver will return an error for any S3 event notifications.
	//
	// If you have objects with multiple different encodings to handle, you should deploy
	// separate Lambda functions with different configurations.
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
