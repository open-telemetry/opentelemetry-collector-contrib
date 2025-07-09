// Copyright The OpenTelemetry Authors
// Portions of this file Copyright 2018-2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package awsutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"

// AWSSessionSettings defines the common session configs for AWS components
type AWSSessionSettings struct {
	// Maximum number of concurrent calls to AWS X-Ray to upload documents.
	NumberOfWorkers int `mapstructure:"num_workers"`
	// X-Ray service endpoint to which the collector sends segment documents.
	Endpoint string `mapstructure:"endpoint"`
	// Number of seconds before timing out a request.
	RequestTimeoutSeconds int `mapstructure:"request_timeout_seconds"`
	// Maximum number of retries before abandoning an attempt to post data.
	MaxRetries int `mapstructure:"max_retries"`
	// Enable or disable TLS certificate verification.
	NoVerifySSL bool `mapstructure:"no_verify_ssl"`
	// Upload segments to AWS X-Ray through a proxy.
	ProxyAddress string `mapstructure:"proxy_address"`
	// Send segments to AWS X-Ray service in a specific region.
	Region string `mapstructure:"region"`
	// Local mode to skip EC2 instance metadata check.
	LocalMode bool `mapstructure:"local_mode"`
	// Amazon Resource Name (ARN) of the AWS resource running the collector.
	ResourceARN string `mapstructure:"resource_arn"`
	// IAM role to upload segments to a different account.
	RoleARN string `mapstructure:"role_arn"`
	// External ID to verify third party role assumption
	ExternalID string `mapstructure:"external_id"`
}

func CreateDefaultSessionConfig() AWSSessionSettings {
	return AWSSessionSettings{
		NumberOfWorkers:       8,
		Endpoint:              "",
		RequestTimeoutSeconds: 30,
		MaxRetries:            2,
		NoVerifySSL:           false,
		ProxyAddress:          "",
		Region:                "",
		LocalMode:             false,
		ResourceARN:           "",
		RoleARN:               "",
	}
}
