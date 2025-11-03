// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redfishreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redfishreceiver"

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redfishreceiver/internal/metadata"
)

type redfishConfig struct {
	Version string `mapstructure:"version"`
}

type Server struct {
	Host             string              `mapstructure:"host"`
	User             string              `mapstructure:"username"`
	Pwd              configopaque.String `mapstructure:"password"`
	Insecure         bool                `mapstructure:"insecure"`
	Timeout          string              `mapstructure:"timeout"`
	Redfish          redfishConfig       `mapstructure:"redfish"`
	ComputerSystemID string              `mapstructure:"computer_system_id"`
	Resources        []Resource          `mapstructure:"resources"`
}

type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	metadata.MetricsBuilderConfig  `mapstructure:",squash"`
	Servers                        []Server `mapstructure:"servers"`

	// prevent unkeyed literal initialization
	_ struct{}
}

func (cfg *Config) Validate() error {
	if len(cfg.Servers) == 0 {
		return errors.New("servers must not be empty")
	}

	for i := range cfg.Servers {
		if cfg.Servers[i].Redfish.Version != "v1" {
			return errors.New("redfish version must be once of the following values: 'v1'")
		}
		if len(cfg.Servers[i].Resources) == 0 {
			return errors.New("resources must not be empty")
		}
		if _, err := time.ParseDuration(cfg.Servers[i].Timeout); err != nil && cfg.Servers[i].Timeout != "" {
			return errors.New("invalid server timeout")
		}
	}
	return nil
}
