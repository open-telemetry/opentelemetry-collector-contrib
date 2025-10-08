// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redfishreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redfishreceiver"

import (
	"fmt"
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
	ComputerSystemId string              `mapstructure:"computer_system_id"`
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
		return fmt.Errorf("servers must not be empty")
	}

	for _, server := range cfg.Servers {
		if server.Redfish.Version != "v1" {
			return fmt.Errorf("redfish version must be once of the following values: 'v1'")
		}
		if len(server.Resources) == 0 {
			return fmt.Errorf("resources must not be empty")
		}
		if _, err := time.ParseDuration(server.Timeout); err != nil && server.Timeout != "" {
			return fmt.Errorf("invalid server timeout")
		}
	}
	return nil
}
