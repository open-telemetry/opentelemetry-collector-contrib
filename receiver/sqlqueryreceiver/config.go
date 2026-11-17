// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlqueryreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlqueryreceiver"

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/config/configdbauth"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sqlquery"
)

type Config struct {
	Config sqlquery.Config `mapstructure:",squash"`
	// DBAuth references an extension that supplies database credentials when
	// opening PostgreSQL connections. It cannot be combined with a datasource
	// string or a static password.
	DBAuth configdbauth.ID `mapstructure:"db_auth,omitempty"`
	// The maximumn number of open connections to the sql server. <= 0 means unlimited
	MaxOpenConn int `mapstructure:"max_open_conn"`

	// prevent unkeyed literal initialization
	_ struct{}
}

func (cfg *Config) Validate() error {
	if err := cfg.Config.Validate(); err != nil {
		return err
	}
	if cfg.DBAuth.IsEmpty() {
		return nil
	}
	if cfg.Config.Driver != sqlquery.DriverPostgres {
		return errors.New("'db_auth' is currently supported only with the 'postgres' driver")
	}
	if cfg.Config.DataSource != "" {
		return errors.New("'db_auth' cannot be set when 'datasource' is specified")
	}
	if cfg.Config.Password != "" {
		return errors.New("set either 'db_auth' or 'password', not both")
	}
	if cfg.Config.Username == "" {
		return errors.New("'username' must be set when 'db_auth' is configured")
	}
	return nil
}

func createDefaultConfig() component.Config {
	cfg := scraperhelper.NewDefaultControllerConfig()
	cfg.CollectionInterval = 10 * time.Second
	return &Config{
		Config: sqlquery.Config{
			ControllerConfig: cfg,
		},
	}
}
