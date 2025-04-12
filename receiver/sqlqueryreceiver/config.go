// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlqueryreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlqueryreceiver"

import (
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sqlquery"
)

type Config struct {
	sqlquery.Config `mapstructure:",squash"`
	// The maximumn number of open connections to the sql server. <= 0 means unlimited
	MaxOpenConn int `mapstructure:"max_open_conn"`
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
