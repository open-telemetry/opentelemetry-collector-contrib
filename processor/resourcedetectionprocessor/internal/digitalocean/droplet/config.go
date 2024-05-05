// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package droplet // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/digitalocean/droplet"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/digitalocean/droplet/internal/metadata"
)

// Config defines user-specified configurations unique to the droplet detector
type Config struct {
	// Tags is a list of regex's to match droplet instance tag keys that users want
	// to add as resource attributes to processed data
	Tags               []string                          `mapstructure:"tags"`
	ResourceAttributes metadata.ResourceAttributesConfig `mapstructure:"resource_attributes"`
}

func CreateDefaultConfig() Config {
	return Config{
		Tags:               []string{},
		ResourceAttributes: metadata.DefaultResourceAttributesConfig(),
	}
}
