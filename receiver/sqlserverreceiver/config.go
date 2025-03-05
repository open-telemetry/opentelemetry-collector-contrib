// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlserverreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver"

import (
	"errors"

	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver/internal/metadata"
)

type QuerySample struct {
	EnableQuerySample bool `mapstructure:"enabled"`
	// MaxCachedQuerySample is the maximum number of query samples that will be cached.
	// Only query that is not in the cache will be reported as sample.
	MaxCachedQuerySample uint `mapstructure:"max_cached_query_sample"`
}

type LogsConfig struct {
	QuerySample `mapstructure:"query_sample"`
}

// Config defines configuration for a sqlserver receiver.
type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	metadata.MetricsBuilderConfig  `mapstructure:",squash"`
	LogsConfig                     `mapstructure:"logs"`

	InstanceName string `mapstructure:"instance_name"`
	ComputerName string `mapstructure:"computer_name"`

	// The following options currently do nothing. Functionality will be added in a future PR.
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

	if cfg.MaxCachedQuerySample > 10000 {
		return errors.New("`max_query_sample_count` must be between 0 and 10000")
	}

	if !directDBConnectionEnabled(cfg) {
		if cfg.Server != "" || cfg.Username != "" || string(cfg.Password) != "" {
			return errors.New("Found one or more of the following configuration options set: [server, port, username, password]. " +
				"All of these options must be configured to directly connect to a SQL Server instance.")
		}
	}

	return nil
}
