// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package osqueryreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/osqueryreceiver"

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
)

const (
	defaultSocket = "/var/osquery/osquery.em"
)

func createDefaultConfig() component.Config {
	scs := scraperhelper.NewDefaultControllerConfig()
	scs.CollectionInterval = 30 * time.Second

	return &Config{
		ExtensionsSocket: defaultSocket,
		ControllerConfig: scs,
	}
}

type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	ExtensionsSocket               string   `mapstructure:"extensions_socket"`
	Queries                        []string `mapstructure:"queries"`
}

func (c Config) Validate() error {
	if len(c.Queries) == 0 {
		return errors.New("queries cannot be empty")
	}
	return nil
}
