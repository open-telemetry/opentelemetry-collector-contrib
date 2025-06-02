// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package eks // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/eks"

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws/retry"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/eks/internal/metadata"
)

type Config struct {
	ResourceAttributes metadata.ResourceAttributesConfig `mapstructure:"resource_attributes"`
	NodeFromEnvVar     string                            `mapstructure:"node_from_env_var"`
	MaxAttempts        int                               `mapstructure:"max_attempts"`
	MaxBackoff         time.Duration                     `mapstructure:"max_backoff"`
}

func CreateDefaultConfig() Config {
	return Config{
		ResourceAttributes: metadata.DefaultResourceAttributesConfig(),
		MaxBackoff:         retry.DefaultMaxBackoff,
		MaxAttempts:        retry.DefaultMaxAttempts,
	}
}
