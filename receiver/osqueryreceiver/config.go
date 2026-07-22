// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package osqueryreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/osqueryreceiver"

import (
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
)

const (
	defaultSocket = "/var/osquery/osquery.em"
)

// validCollections defines the allowed values for Config.Collections.
var validCollections = map[string]struct{}{
	"system_info":     {},
	"package_info":    {},
	"os_info":         {},
	"secureboot_info": {},
	"users_info":      {},
}

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
	// Collections lists predefined, named osquery collections to run in addition to Queries.
	// See validCollections for the allowed values.
	Collections []string `mapstructure:"collections"`
}

func (c Config) Validate() error {
	if len(c.Queries) == 0 && len(c.Collections) == 0 {
		return errors.New("either queries or collections must be specified")
	}

	for _, name := range c.Collections {
		if _, ok := validCollections[name]; !ok {
			return fmt.Errorf("invalid collection %q: must be one of %s", name, validCollections)
		}
	}

	return nil
}
