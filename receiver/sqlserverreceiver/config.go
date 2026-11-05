// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlserverreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver"

import (
	"errors"
	"slices"
	"time"

	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/config/configdbauth"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver/internal/metadata"
)

type QuerySample struct {
	MaxRowsPerQuery uint64 `mapstructure:"max_rows_per_query"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// ConnectionPool configures the shared database connection pool used by all
// scrapers of the receiver. Any field left unset falls back to a default
// derived from the number of scrapers (see setConnectionPoolSettings).
type ConnectionPool struct {
	// MaxIdleTime is the maximum amount of time a connection may be idle before
	// being closed. Zero means connections are not closed due to idle time.
	MaxIdleTime *time.Duration `mapstructure:"max_idle_time,omitempty"`
	// MaxLifetime is the maximum amount of time a connection may be reused.
	// Zero means connections are reused forever.
	MaxLifetime *time.Duration `mapstructure:"max_lifetime,omitempty"`
	// MaxIdle is the maximum number of idle connections kept in the pool.
	MaxIdle *int `mapstructure:"max_idle,omitempty"`
	// MaxOpen is the maximum number of open connections to the database.
	// Zero means unlimited.
	MaxOpen *int `mapstructure:"max_open,omitempty"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// TopProcedureCollection configures the collection of stored procedure statistics. Candidate
// rows are fetched up to MaxProcedureSampleCount, then the TopProcedureCount procedures
// with the largest elapsed-time delta over the interval are reported.
type TopProcedureCollection struct {
	MaxProcedureSampleCount uint `mapstructure:"max_procedure_sample_count"`
	TopProcedureCount       uint `mapstructure:"top_procedure_count"`
	// CollectionInterval throttles the query and sets the window the deltas cover.
	CollectionInterval time.Duration `mapstructure:"collection_interval"`

	// prevent unkeyed literal initialization
	_ struct{}
}

type TopQueryCollection struct {
	// Enabled enables the collection of the top queries by the execution time.
	// It will collect the top N queries based on totalElapsedTimeDiffs during the last collection interval.
	// The query statement will also be reported, hence, it is not ideal to send it as a metric. Hence
	// we are reporting them as logs.
	// The `N` is configured via `TopQueryCount`
	LookbackTime        time.Duration `mapstructure:"lookback_time"`
	MaxQuerySampleCount uint          `mapstructure:"max_query_sample_count"`
	TopQueryCount       uint          `mapstructure:"top_query_count"`
	CollectionInterval  time.Duration `mapstructure:"collection_interval"`
}

// Config defines configuration for a sqlserver receiver.
type Config struct {
	ControllerConfig     scraperhelper.ControllerConfig `mapstructure:",squash"`
	MetricsBuilderConfig metadata.MetricsBuilderConfig  `mapstructure:",squash"`
	LogsBuilderConfig    metadata.LogsBuilderConfig     `mapstructure:",squash"`
	// EnableTopQueryCollection enables the collection of the top queries by the execution time.
	// It will collect the top N queries based on totalElapsedTimeDiffs during the last collection interval.
	// The query statement will also be reported, hence, it is not ideal to send it as a metric. Hence
	// we are reporting them as logs.
	// The `N` is configured via `TopQueryCount`
	TopQueryCollection TopQueryCollection `mapstructure:"top_query_collection"`

	QuerySample QuerySample `mapstructure:"query_sample_collection"`

	TopProcedureCollection TopProcedureCollection `mapstructure:"top_procedure_collection"`

	// ConnectionPool tunes the shared database connection pool used by all
	// scrapers of this receiver.
	ConnectionPool ConnectionPool `mapstructure:"connection_pool,omitempty"`

	// Databases limits the per-database metrics and the events to the listed
	// databases. Empty reports on every database.
	Databases []string `mapstructure:"databases,omitempty"`
	// ExcludeDatabases lists databases left out of the per-database metrics and
	// the events. A database in both lists is excluded.
	ExcludeDatabases []string `mapstructure:"exclude_databases,omitempty"`

	InstanceName string `mapstructure:"instance_name"`
	ComputerName string `mapstructure:"computer_name"`

	DataSource string `mapstructure:"datasource"`

	// DBAuth optionally references a database credential provider extension.
	// When set, the provider supplies the secret at connection time. Username
	// remains available as provider input; a static password cannot be set.
	DBAuth configdbauth.ID `mapstructure:"db_auth,omitempty"`

	Password configopaque.String `mapstructure:"password"`
	Port     uint                `mapstructure:"port"`
	Server   string              `mapstructure:"server"`
	Username string              `mapstructure:"username"`

	// Flag to check if the connection is direct or not. It should only be
	// used after a successful call to the `Validate` method.
	isDirectDBConnectionEnabled bool
}

func (cfg *Config) Validate() error {
	err := cfg.validateInstanceAndComputerName()
	if err != nil {
		return err
	}

	if cfg.TopQueryCollection.LookbackTime < 0 {
		return errors.New("lookback_time cannot have negative values")
	}

	if cfg.TopQueryCollection.MaxQuerySampleCount > 10000 {
		return errors.New("`max_query_sample_count` must be between 0 and 10000")
	}

	if cfg.TopQueryCollection.TopQueryCount > cfg.TopQueryCollection.MaxQuerySampleCount {
		return errors.New("`top_query_count` must be less than or equal to `max_query_sample_count`")
	}

	if cfg.TopQueryCollection.CollectionInterval < 0 {
		return errors.New("`top_query_collection.collection_interval` must not be less than 0")
	}

	if cfg.TopProcedureCollection.MaxProcedureSampleCount > 10000 {
		return errors.New("`max_procedure_sample_count` must be between 1 and 10000")
	}

	if cfg.TopProcedureCollection.TopProcedureCount > cfg.TopProcedureCollection.MaxProcedureSampleCount {
		return errors.New("`top_procedure_count` must be less than or equal to `max_procedure_sample_count`")
	}

	if cfg.TopProcedureCollection.CollectionInterval < 0 {
		return errors.New("`top_procedure_collection.collection_interval` must not be less than 0")
	}

	// Zero counts would leave the event silently reporting nothing, so they are only
	// rejected when the event that reads them is actually enabled.
	if cfg.LogsBuilderConfig.Events.DbServerTopProcedure.Enabled {
		if cfg.TopProcedureCollection.MaxProcedureSampleCount < 1 {
			return errors.New("`max_procedure_sample_count` must be between 1 and 10000")
		}
		if cfg.TopProcedureCollection.TopProcedureCount < 1 {
			return errors.New("`top_procedure_count` must be greater than 0")
		}
	}

	if poolErr := cfg.validateConnectionPool(); poolErr != nil {
		return poolErr
	}

	if slices.Contains(cfg.Databases, "") {
		return errors.New("`databases` must not contain an empty name")
	}

	if slices.Contains(cfg.ExcludeDatabases, "") {
		return errors.New("`exclude_databases` must not contain an empty name")
	}

	cfg.isDirectDBConnectionEnabled, err = directDBConnectionEnabled(cfg)

	return err
}

func (cfg *Config) validateConnectionPool() error {
	pool := cfg.ConnectionPool
	if pool.MaxOpen != nil && *pool.MaxOpen < 0 {
		return errors.New("`connection_pool.max_open` must not be negative")
	}
	if pool.MaxIdle != nil && *pool.MaxIdle < 0 {
		return errors.New("`connection_pool.max_idle` must not be negative")
	}
	if pool.MaxLifetime != nil && *pool.MaxLifetime < 0 {
		return errors.New("`connection_pool.max_lifetime` must not be negative")
	}
	if pool.MaxIdleTime != nil && *pool.MaxIdleTime < 0 {
		return errors.New("`connection_pool.max_idle_time` must not be negative")
	}
	return nil
}

func directDBConnectionEnabled(config *Config) (bool, error) {
	hasDataSource := config.DataSource != ""
	hasDBAuth := !config.DBAuth.IsEmpty()
	noneOfServerUserPasswordPortSet := config.Server == "" && config.Username == "" && string(config.Password) == "" && config.Port == 0

	if !hasDataSource && noneOfServerUserPasswordPortSet && !hasDBAuth {
		// If no connection information is provided, we can't connect directly and this is a valid config.
		return false, nil
	}

	anyOfServerUserPasswordPortSet := config.Server != "" || config.Username != "" || string(config.Password) != "" || config.Port != 0
	if hasDataSource && anyOfServerUserPasswordPortSet {
		return false, errors.New("wrong config: when specifying 'datasource' no other connection parameters ('server', 'username', 'password', or 'port') should be set")
	}

	if hasDBAuth {
		if string(config.Password) != "" {
			return false, errors.New("wrong config: set either 'password' or 'db_auth', not both")
		}
		// Without a datasource we need at least server and port to build a DSN.
		if !hasDataSource && (config.Server == "" || config.Port == 0) {
			return false, errors.New("wrong config: when using 'db_auth' without 'datasource', both 'server' and 'port' must be specified")
		}
		return true, nil
	}

	if !hasDataSource && (config.Server == "" || config.Username == "" || string(config.Password) == "" || config.Port == 0) {
		return false, errors.New("wrong config: when specifying either 'server', 'username', 'password', or 'port' all of them need to be specified")
	}

	// It is a valid direct connection configuration
	return true, nil
}

func (cfg *Config) EffectiveLookbackTime() time.Duration {
	if cfg.TopQueryCollection.LookbackTime == 0 {
		return 2 * cfg.TopQueryCollection.CollectionInterval
	}
	return cfg.TopQueryCollection.LookbackTime
}
