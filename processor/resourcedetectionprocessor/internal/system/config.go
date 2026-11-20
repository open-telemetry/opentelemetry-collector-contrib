// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package system // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/system"

import (
	"fmt"

	"go.opentelemetry.io/collector/confmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/system/internal/metadata"
)

var validHostnameSources = map[string]struct{}{
	"os":     {},
	"dns":    {},
	"cname":  {},
	"lookup": {},
}

// Config defines user-specified configurations unique to the system detector
type Config struct {
	// The HostnameSources is a priority list of sources from which hostname will be fetched.
	// In case of the error in fetching hostname from source,
	// the next source from the list will be considered.(**default**: `["dns", "os"]`)
	HostnameSources []string `mapstructure:"hostname_sources"`

	ResourceAttributes metadata.ResourceAttributesConfig `mapstructure:"resource_attributes"`

	deprecatedOSBuildID metadata.ResourceAttributeConfig
}

var deprecatedOSBuildIDKey = "resource_attributes" + confmap.KeyDelimiter + "os.build.id"

// Unmarshal reads the deprecated os.build.id key separately, since it is not part of the generated config.
func (cfg *Config) Unmarshal(conf *confmap.Conf) error {
	if conf == nil {
		return nil
	}
	if conf.IsSet(deprecatedOSBuildIDKey) {
		sub, err := conf.Sub(deprecatedOSBuildIDKey)
		if err != nil {
			return err
		}
		if err := sub.Unmarshal(&cfg.deprecatedOSBuildID); err != nil {
			return err
		}
		conf.Delete(deprecatedOSBuildIDKey)
	}
	return conf.Unmarshal(cfg)
}

// Validate config
func (cfg *Config) Validate() error {
	for _, hostnameSource := range cfg.HostnameSources {
		_, exists := validHostnameSources[hostnameSource]
		if !exists {
			return fmt.Errorf("hostname_sources contains invalid value: %q", hostnameSource)
		}
	}
	return nil
}

func CreateDefaultConfig() Config {
	return Config{
		ResourceAttributes: metadata.DefaultResourceAttributesConfig(),
	}
}
