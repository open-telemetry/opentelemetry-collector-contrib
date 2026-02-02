// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redfishreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redfishreceiver"

import (
	"errors"
	"net/url"
	"strings"
	"time"

	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redfishreceiver/internal/metadata"
)

type redfishConfig struct {
	Version string `mapstructure:"version"`
}

type Server struct {
	BaseURL          string              `mapstructure:"base_url"`
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
		if _, err := url.ParseRequestURI(cfg.Servers[i].BaseURL); err != nil {
			return err
		}

		if strings.TrimSpace(cfg.Servers[i].ComputerSystemID) == "" {
			return errors.New("computer_system_id must not be empty")
		}

		if _, err := time.ParseDuration(cfg.Servers[i].Timeout); err != nil && cfg.Servers[i].Timeout != "" {
			return errors.New("invalid server timeout")
		}

		if len(cfg.Servers[i].Resources) == 0 {
			return errors.New("resources must not be empty")
		}
	}
	return nil
}
