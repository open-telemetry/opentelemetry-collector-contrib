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
	// See validCollections for the allowed values. Unlike Queries, each collection's rows are
	// diffed against its last-known state, so only new or modified rows are emitted.
	Collections []string `mapstructure:"collections"`
	// SnapshotInterval, if set, additionally emits the full current state of all configured
	// Collections on this interval, regardless of whether anything changed. Disabled by default.
	SnapshotInterval time.Duration `mapstructure:"snapshot_interval"`
	// StorageID names a storage extension to persist each collection's last-known rows across
	// restarts, so change detection produces correct diffs instead of re-emitting everything
	// after a restart. If unset, diffing still works within a single collector run, but the
	// state is lost on restart.
	StorageID *component.ID `mapstructure:"storage"`
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

	if c.SnapshotInterval < 0 {
		return errors.New("snapshot_interval must not be negative")
	}

	return nil
}
