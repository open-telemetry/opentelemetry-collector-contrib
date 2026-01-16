// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package scaleway // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/scaleway"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/scaleway/internal/metadata"
)

type Config struct {
	ResourceAttributes metadata.ResourceAttributesConfig `mapstructure:"resource_attributes"`
}

func CreateDefaultConfig() Config {
	return Config{
		ResourceAttributes: metadata.DefaultResourceAttributesConfig(),
	}
}
