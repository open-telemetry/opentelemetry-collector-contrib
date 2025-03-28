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
	Enabled         bool   `mapstructure:"enabled"`
	MaxRowsPerQuery uint64 `mapstructure:"max_rows_per_query"`
}

type TopQueryCollection struct {
	// Enabled enables the collection of the top queries by the execution time.
	// It will collect the top N queries based on totalElapsedTimeDiffs during the last collection interval.
	// The query statement will also be reported, hence, it is not ideal to send it as a metric. Hence
	// we are reporting them as logs.
	// The `N` is configured via `TopQueryCount`
	Enabled             bool `mapstructure:"enabled"`
	LookbackTime        uint `mapstructure:"lookback_time"`
	MaxQuerySampleCount uint `mapstructure:"max_query_sample_count"`
	TopQueryCount       uint `mapstructure:"top_query_count"`
}

// Config defines configuration for a sqlserver receiver.
type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	metadata.MetricsBuilderConfig  `mapstructure:",squash"`
	// EnableTopQueryCollection enables the collection of the top queries by the execution time.
	// It will collect the top N queries based on totalElapsedTimeDiffs during the last collection interval.
	// The query statement will also be reported, hence, it is not ideal to send it as a metric. Hence
	// we are reporting them as logs.
	// The `N` is configured via `TopQueryCount`
	TopQueryCollection `mapstructure:"top_query_collection"`

	QuerySample `mapstructure:"query_sample_collection"`

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

	if cfg.TopQueryCollection.MaxQuerySampleCount > 10000 {
		return errors.New("`max_query_sample_count` must be between 0 and 10000")
	}

	if cfg.TopQueryCount > cfg.TopQueryCollection.MaxQuerySampleCount {
		return errors.New("`top_query_count` must be less than or equal to `max_query_sample_count`")
	}

	if !directDBConnectionEnabled(cfg) {
		if cfg.Server != "" || cfg.Username != "" || string(cfg.Password) != "" {
			return errors.New("Found one or more of the following configuration options set: [server, port, username, password]. " +
				"All of these options must be configured to directly connect to a SQL Server instance.")
		}
	}

	return nil
}
