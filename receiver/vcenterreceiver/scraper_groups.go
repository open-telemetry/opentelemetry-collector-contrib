// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vcenterreceiver

type ScraperGroup string

const (
	ScraperGroupCluster      ScraperGroup = "cluster"
	ScraperGroupHost         ScraperGroup = "host"
	ScraperGroupVM           ScraperGroup = "vm"
	ScraperGroupDatastore    ScraperGroup = "datastore"
	ScraperGroupResourcePool ScraperGroup = "resourcepool"
	ScraperGroupDatacenter   ScraperGroup = "datacenter"
	ScraperGroupVSAN         ScraperGroup = "vsan"
)

// AllScraperGroups returns all available scraper groups
func AllScraperGroups() []ScraperGroup {
	return []ScraperGroup{
		ScraperGroupCluster,
		ScraperGroupHost,
		ScraperGroupVM,
		ScraperGroupDatastore,
		ScraperGroupResourcePool,
		ScraperGroupDatacenter,
		ScraperGroupVSAN,
	}
}

// isScraperGroupEnabled checks if a scraper group is enabled in the config.
// If the scraper group is not specified in the config, it defaults to enabled
// for backward compatibility.
func isScraperGroupEnabled(cfg *Config, group ScraperGroup) bool {
	if cfg.Scrapers == nil {
		return true
	}
	scraperCfg, exists := cfg.Scrapers[group]
	if !exists {
		return true
	}
	return scraperCfg.Enabled
}
