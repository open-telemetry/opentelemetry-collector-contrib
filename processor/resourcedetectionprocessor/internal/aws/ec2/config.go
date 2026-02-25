// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ec2 // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/ec2"

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws/retry"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/ec2/internal/metadata"
)

// Config defines user-specified configurations unique to the EC2 detector
type Config struct {
	// Tags is a list of regex's to match ec2 instance tag keys that users want
	// to add as resource attributes to processed data
	Tags                  []string                          `mapstructure:"tags"`
	ResourceAttributes    metadata.ResourceAttributesConfig `mapstructure:"resource_attributes"`
	MaxAttempts           int                               `mapstructure:"max_attempts"`
	MaxBackoff            time.Duration                     `mapstructure:"max_backoff"`
	FailOnMissingMetadata bool                              `mapstructure:"fail_on_missing_metadata"`
}

func CreateDefaultConfig() Config {
	return Config{
		Tags:               []string{},
		ResourceAttributes: metadata.DefaultResourceAttributesConfig(),
		MaxAttempts:        retry.DefaultMaxAttempts,
		MaxBackoff:         retry.DefaultMaxBackoff,
	}
}
