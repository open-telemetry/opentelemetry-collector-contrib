// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package postgresqlreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver"

import (
	"errors"
	"fmt"
	"net"
	"time"

	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver/internal/metadata"
)

// Errors for missing required config parameters.
const (
	ErrNoUsername          = "invalid config: missing username"
	ErrNoPassword          = "invalid config: missing password" // #nosec G101 - not hardcoded credentials
	ErrNotSupported        = "invalid config: field '%s' not supported"
	ErrTransportsSupported = "invalid config: 'transport' must be 'tcp' or 'unix'"
	ErrHostPort            = "invalid config: 'endpoint' must be in the form <host>:<port> no matter what 'transport' is configured"
)

type TopQueryCollection struct {
	Enabled         bool  `mapstructure:"enabled"`
	MaxRowsPerQuery int64 `mapstructure:"max_rows_per_query"`
	TopNQuery       int64 `mapstructure:"top_n_query"`
}

type QuerySampleCollection struct {
	Enabled         bool  `mapstructure:"enabled"`
	MaxRowsPerQuery int64 `mapstructure:"max_rows_per_query"`
}

type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	Username                       string                         `mapstructure:"username"`
	Password                       configopaque.String            `mapstructure:"password"`
	Databases                      []string                       `mapstructure:"databases"`
	ExcludeDatabases               []string                       `mapstructure:"exclude_databases"`
	confignet.AddrConfig           `mapstructure:",squash"`       // provides Endpoint and Transport
	configtls.ClientConfig         `mapstructure:"tls,omitempty"` // provides SSL details
	ConnectionPool                 `mapstructure:"connection_pool,omitempty"`
	metadata.MetricsBuilderConfig  `mapstructure:",squash"`
	QuerySampleCollection          `mapstructure:"query_sample_collection,omitempty"`
	TopQueryCollection             `mapstructure:"top_query_collection,omitempty"`
}

type ConnectionPool struct {
	MaxIdleTime *time.Duration `mapstructure:"max_idle_time,omitempty"`
	MaxLifetime *time.Duration `mapstructure:"max_lifetime,omitempty"`
	MaxIdle     *int           `mapstructure:"max_idle,omitempty"`
	MaxOpen     *int           `mapstructure:"max_open,omitempty"`
}

func (cfg *Config) Validate() error {
	var err error
	if cfg.Username == "" {
		err = multierr.Append(err, errors.New(ErrNoUsername))
	}
	if cfg.Password == "" {
		err = multierr.Append(err, errors.New(ErrNoPassword))
	}

	// The lib/pq module does not support overriding ServerName or specifying supported TLS versions
	if cfg.ServerName != "" {
		err = multierr.Append(err, fmt.Errorf(ErrNotSupported, "ServerName"))
	}
	if cfg.MaxVersion != "" {
		err = multierr.Append(err, fmt.Errorf(ErrNotSupported, "MaxVersion"))
	}
	if cfg.MinVersion != "" {
		err = multierr.Append(err, fmt.Errorf(ErrNotSupported, "MinVersion"))
	}

	switch cfg.Transport {
	case confignet.TransportTypeTCP, confignet.TransportTypeUnix:
		_, _, endpointErr := net.SplitHostPort(cfg.Endpoint)
		if endpointErr != nil {
			err = multierr.Append(err, errors.New(ErrHostPort))
		}
	default:
		err = multierr.Append(err, errors.New(ErrTransportsSupported))
	}

	return err
}
