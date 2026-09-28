// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mysqlreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver"

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/config/configdbauth"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/dbauth"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver/internal/metadata"
)

const (
	defaultStatementEventsDigestTextLimit = 120
	defaultStatementEventsLimit           = 250
	defaultStatementEventsTimeLimit       = 24 * time.Hour
)

// Errors for missing required config parameters.
const (
	ErrNoUsername          = "invalid config: missing username"
	ErrTransportsSupported = "invalid config: 'transport' must be 'tcp' or 'unix'"
	ErrNoEndpoint          = "invalid config: missing endpoint"
	// #nosec G101 - not hardcoded credentials
	ErrPasswordAndDBAuth = "invalid config: set either 'password' or 'db_auth', not both"
	// #nosec G101 - not hardcoded credentials
	ErrDBAuthRequiresTLS = "invalid config: 'db_auth' requires TLS; set 'tls.insecure' to false"
)

type Config struct {
	ControllerConfig      scraperhelper.ControllerConfig `mapstructure:",squash"`
	Username              string                         `mapstructure:"username,omitempty"`
	Password              configopaque.String            `mapstructure:"password,omitempty"`
	Database              string                         `mapstructure:"database,omitempty"`
	AllowNativePasswords  bool                           `mapstructure:"allow_native_passwords,omitempty"`
	AddrConfig            confignet.AddrConfig           `mapstructure:",squash"`
	TLS                   configtls.ClientConfig         `mapstructure:"tls,omitempty"`
	MetricsBuilderConfig  metadata.MetricsBuilderConfig  `mapstructure:",squash"`
	LogsBuilderConfig     metadata.LogsBuilderConfig     `mapstructure:",squash"`
	StatementEvents       StatementEventsConfig          `mapstructure:"statement_events"`
	TopQueryCollection    TopQueryCollection             `mapstructure:"top_query_collection"`
	QuerySampleCollection QuerySampleCollection          `mapstructure:"query_sample_collection"`
	DBAuth                configdbauth.ID                `mapstructure:"db_auth,omitempty"`
}

type TopQueryCollection struct {
	LookbackTime        uint64        `mapstructure:"lookback_time"`
	MaxQuerySampleCount uint64        `mapstructure:"max_query_sample_count"`
	TopQueryCount       uint64        `mapstructure:"top_query_count"`
	CollectionInterval  time.Duration `mapstructure:"collection_interval"`
	QueryPlanCacheSize  int           `mapstructure:"query_plan_cache_size"`
	QueryPlanCacheTTL   time.Duration `mapstructure:"query_plan_cache_ttl"`

	_ struct{}
}
type QuerySampleCollection struct {
	MaxRowsPerQuery uint64 `mapstructure:"max_rows_per_query"`

	_ struct{}
}

type StatementEventsConfig struct {
	DigestTextLimit int           `mapstructure:"digest_text_limit"`
	Limit           int           `mapstructure:"limit"`
	TimeLimit       time.Duration `mapstructure:"time_limit"`
}

func (cfg *Config) Unmarshal(componentParser *confmap.Conf) error {
	if componentParser == nil {
		// Nothing to do if there is no config given.
		return nil
	}

	// Change the default to Insecure = true as we don't want to break
	// existing deployments which does not use TLS by default.
	if !componentParser.IsSet("tls") {
		cfg.TLS = configtls.ClientConfig{}
		cfg.TLS.Insecure = true
	}

	return componentParser.Unmarshal(cfg)
}

func (cfg *Config) Validate() error {
	var err error
	if cfg.Username == "" {
		err = multierr.Append(err, errors.New(ErrNoUsername))
	}

	dbAuthConfigured := !cfg.DBAuth.IsEmpty()
	switch {
	case dbAuthConfigured && cfg.Password != "":
		err = multierr.Append(err, errors.New(ErrPasswordAndDBAuth))
	case dbAuthConfigured && cfg.TLS.Insecure:
		err = multierr.Append(err, errors.New(ErrDBAuthRequiresTLS))
	}

	switch cfg.AddrConfig.Transport {
	case confignet.TransportTypeTCP, confignet.TransportTypeUnix:
		if cfg.AddrConfig.Endpoint == "" {
			err = multierr.Append(err, errors.New(ErrNoEndpoint))
		}
	default:
		err = multierr.Append(err, errors.New(ErrTransportsSupported))
	}

	return err
}

func (cfg *Config) resolveCredentialProvider(extensions map[component.ID]component.Component) (dbauth.Provider, error) {
	if cfg.DBAuth.IsEmpty() {
		return nil, nil
	}
	return cfg.DBAuth.GetProvider(extensions)
}
