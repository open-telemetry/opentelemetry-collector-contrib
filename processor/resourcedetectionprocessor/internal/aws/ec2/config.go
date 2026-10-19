// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ec2 // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/ec2"

import (
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/ec2/internal/metadata"
)

// These mirror aws-sdk-go-v2's retry.DefaultMaxAttempts and retry.DefaultMaxBackoff.
// They are declared locally so that this config package does not import the AWS SDK
// just for two constants, which would pull the SDK into every build's config schema.
// TestDefaultRetrySettingsMatchSDK checks for drift when the SDK is upgraded.
// For maintenance steps, see ../../../README.md#maintaining-the-ec2-retry-defaults:
// https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/resourcedetectionprocessor#maintaining-the-ec2-retry-defaults
const (
	defaultMaxAttempts = 3
	defaultMaxBackoff  = 20 * time.Second
)

// Config defines user-specified configurations unique to the EC2 detector
type Config struct {
	// Tags is a list of regex's to match ec2 instance tag keys that users want
	// to add as resource attributes to processed data
	Tags               []string                          `mapstructure:"tags"`
	ResourceAttributes metadata.ResourceAttributesConfig `mapstructure:"resource_attributes"`
	MaxAttempts        int                               `mapstructure:"max_attempts"`
	MaxBackoff         time.Duration                     `mapstructure:"max_backoff"`
	// Deprecated: Use the top-level fail_on_missing_metadata in the processor config instead.
	// This field will be removed in a future release.
	FailOnMissingMetadata bool `mapstructure:"fail_on_missing_metadata"`
	// TagsFromIMDS controls whether instance tags are fetched via IMDS (true)
	// or via the EC2 DescribeTags API (false, default).
	// IMDS does not require IAM permissions but requires InstanceMetadataTags=enabled on the instance.
	// Note: IMDS must be network-accessible from the process; workloads running in containers
	// (e.g., EKS pods, ECS tasks) may not have access to IMDS even if it is enabled on the host,
	// unless the instance metadata options are configured to allow container access.
	TagsFromIMDS bool `mapstructure:"tags_from_imds"`
}

func CreateDefaultConfig() Config {
	return Config{
		Tags:               []string{},
		ResourceAttributes: metadata.DefaultResourceAttributesConfig(),
		MaxAttempts:        defaultMaxAttempts,
		MaxBackoff:         defaultMaxBackoff,
	}
}
