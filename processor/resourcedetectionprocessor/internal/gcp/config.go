// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gcp // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/gcp"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/gcp/internal/metadata"
)

type Config struct {
	// Labels is a list of regex's to match gce instance label keys that users want
	// to add as resource attributes to processed data
	Labels             []string                          `mapstructure:"labels"`
	ResourceAttributes metadata.ResourceAttributesConfig `mapstructure:"resource_attributes"`
}

func CreateDefaultConfig() Config {
	return Config{
		Labels:             []string{},
		ResourceAttributes: metadata.DefaultResourceAttributesConfig(),
	}
}
