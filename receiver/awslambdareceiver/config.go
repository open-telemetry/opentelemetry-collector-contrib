// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awslambdareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awslambdareceiver"

import (
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/component"
)

const s3ARNPrefix = "arn:aws:s3:::"

type Config struct {
	// S3 defines configuration options for S3 Lambda trigger.
	S3 sharedConfig `mapstructure:"s3"`

	// CloudWatch defines configuration options for CloudWatch Lambda trigger.
	CloudWatch sharedConfig `mapstructure:"cloudwatch"`

	// FailureBucketARN is the ARN of receiver deployment Lambda's error destination.
	FailureBucketARN string `mapstructure:"failure_bucket_arn"`

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

func (c *Config) Validate() error {
	if c.FailureBucketARN != "" {
		_, err := getBucketNameFromARN(c.FailureBucketARN)
		if err != nil {
			return fmt.Errorf("invalid failure_bucket_arn: %w", err)
		}
	}

	return nil
}

// getBucketNameFromARN extracts S3 bucket name from ARN
// Example
//
//	arn = "arn:aws:s3:::myBucket/folderA
//	result = myBucket
func getBucketNameFromARN(arn string) (string, error) {
	if !strings.HasPrefix(arn, s3ARNPrefix) {
		return "", fmt.Errorf("invalid S3 ARN format: %s", arn)
	}

	s3Path := strings.TrimPrefix(arn, s3ARNPrefix)
	bucket, _, _ := strings.Cut(s3Path, "/")

	if bucket == "" {
		return "", fmt.Errorf("invalid S3 ARN format, bucket name missing: %s", arn)
	}

	return bucket, nil
}
