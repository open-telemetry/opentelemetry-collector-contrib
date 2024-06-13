// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlserverreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver"

import (
	"fmt"

	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver/internal/metadata"
)

// Config defines configuration for a sqlserver receiver.
type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	metadata.MetricsBuilderConfig  `mapstructure:",squash"`

	InstanceName string `mapstructure:"instance_name"`
	ComputerName string `mapstructure:"computer_name"`

	Password configopaque.String `mapstructure:"password"`
	Port     uint                `mapstructure:"port"`
	Server   string              `mapstructure:"server"`
	Username string              `mapstructure:"username"`
}

func (cfg *Config) Validate() error {
	err := cfg.validateInstanceAndComputerName()
	if err != nil {
		return err
	}

	if !directDBConnectionEnabled(cfg) {
		if cfg.Server != "" || cfg.Username != "" || string(cfg.Password) != "" {
			return fmt.Errorf("Found one or more of the following configuration options set: [server, username, password]. " +
				"All of these options must be configured to directly connect to a SQL Server instance.")
		}
	} else {
		_, port := getServerPort(cfg)
		// 0 is both an invalid port and the result of casting an empty string to uint
		if port == "" {
			return fmt.Errorf("the connection port of the SQL Server instance must be specified, either in the `server` or `port` configuration option")
		}
	}

	return nil
}
