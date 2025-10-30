// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awslambdareceiver

import (
	"errors"
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/component"
)

const (
	awsLogsEncoding = "awslogs_encoding"
	s3ARNPrefix     = "arn:aws:s3:::"
)

type Config struct {
	// S3Encoding identifies the encoding of the S3 objects that trigger the Lambda.
	//
	// If this is unspecified, the receiver defaults to parsing logs in CloudWatch Log
	// subscription filter format.
	S3Encoding string `mapstructure:"s3_encoding"`

	// FailureBucketARN is the ARN of receiver deployment Lambda's error destination.
	FailureBucketARN string `mapstructure:"failure_bucket_arn"`
}

var _ component.Config = (*Config)(nil)

func createDefaultConfig() component.Config {
	return &Config{}
}

func (c *Config) Validate() error {
	if c.FailureBucketARN == "" {
		return errors.New("failure_bucket_arn must be set")
	}

	_, err := getBucketNameFromARN(c.FailureBucketARN)
	if err != nil {
		return fmt.Errorf("invalid failure_bucket_arn: %w", err)
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
	pathParts := strings.SplitN(s3Path, "/", 2)

	if pathParts[0] == "" {
		return "", fmt.Errorf("invalid S3 ARN format, bucket name missing: %s", arn)
	}

	return pathParts[0], nil
}
