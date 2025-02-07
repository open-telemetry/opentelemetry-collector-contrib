// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlserverreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver"

import (
	"errors"

	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver/internal/metadata"
)

// Config defines configuration for a sqlserver receiver.
type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	metadata.MetricsBuilderConfig  `mapstructure:",squash"`

	InstanceName string `mapstructure:"instance_name"`
	ComputerName string `mapstructure:"computer_name"`

	// The following options currently do nothing. Functionality will be added in a future PR.
	Password            configopaque.String `mapstructure:"password"`
	Port                uint                `mapstructure:"port"`
	Server              string              `mapstructure:"server"`
	Username            string              `mapstructure:"username"`
	Granularity         uint                `mapstructure:"granularity"`
	MaxQuerySampleCount uint                `mapstructure:"max_query_sample_count"`
	TopQueryCount       uint                `mapstructure:"top_query_count"`
}

func (cfg *Config) Validate() error {
	err := cfg.validateInstanceAndComputerName()
	if err != nil {
		return err
	}

	if cfg.MaxQuerySampleCount > 10000 {
		return errors.New("`max_query_sample_count` must be between 0 and 10000")
	}

	if cfg.TopQueryCount > 200 || cfg.TopQueryCount > cfg.MaxQuerySampleCount {
		return errors.New("`top_query_count` must be between 0 and 200 and less than or equal to `max_query_sample_count`")
	}

	if !directDBConnectionEnabled(cfg) {
		if cfg.Server != "" || cfg.Username != "" || string(cfg.Password) != "" {
			return errors.New("Found one or more of the following configuration options set: [server, port, username, password]. " +
				"All of these options must be configured to directly connect to a SQL Server instance.")
		}
	}

	return nil
}
