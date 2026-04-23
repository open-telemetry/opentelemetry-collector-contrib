// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vultr // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/vultr"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/vultr/internal/metadata"
)

type Config struct {
	ResourceAttributes    metadata.ResourceAttributesConfig `mapstructure:"resource_attributes"`
	FailOnMissingMetadata bool                              `mapstructure:"fail_on_missing_metadata"`
}

func CreateDefaultConfig() Config {
	return Config{
		ResourceAttributes: metadata.DefaultResourceAttributesConfig(),
	}
}
