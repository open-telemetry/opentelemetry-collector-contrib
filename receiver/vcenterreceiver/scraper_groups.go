// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vcenterreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver"

import (
	"reflect"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver/internal/metadata"
)

// ScraperGroup identifies a group of metrics that can be enabled/disabled together.
type ScraperGroup string

const (
	ScraperGroupCluster      ScraperGroup = "cluster"
	ScraperGroupDatacenter   ScraperGroup = "datacenter"
	ScraperGroupDatastore    ScraperGroup = "datastore"
	ScraperGroupHost         ScraperGroup = "host"
	ScraperGroupResourcePool ScraperGroup = "resourcepool"
	ScraperGroupVM           ScraperGroup = "vm"
	ScraperGroupVSAN         ScraperGroup = "vsan"
)

func allScraperGroups() []ScraperGroup {
	return []ScraperGroup{
		ScraperGroupCluster,
		ScraperGroupDatacenter,
		ScraperGroupDatastore,
		ScraperGroupHost,
		ScraperGroupResourcePool,
		ScraperGroupVM,
		ScraperGroupVSAN,
	}
}

func validScraperGroupSet() map[ScraperGroup]bool {
	set := make(map[ScraperGroup]bool)
	for _, g := range allScraperGroups() {
		set[g] = true
	}
	return set
}

type ScraperConfig struct {
	Enabled bool `mapstructure:"enabled"`
	// prevent unkeyed literal initialization
	_ struct{}
}

// groupForMetricName returns the scraper group for a metric by its mapstructure name
// (e.g. "vcenter.cluster.vsan.throughput" -> ScraperGroupVSAN).
func groupForMetricName(name string) ScraperGroup {
	if strings.Contains(name, ".vsan.") || strings.HasSuffix(name, ".vsan") {
		return ScraperGroupVSAN
	}
	switch {
	case strings.HasPrefix(name, "vcenter.cluster."):
		return ScraperGroupCluster
	case strings.HasPrefix(name, "vcenter.datacenter."):
		return ScraperGroupDatacenter
	case strings.HasPrefix(name, "vcenter.datastore."):
		return ScraperGroupDatastore
	case strings.HasPrefix(name, "vcenter.host."):
		return ScraperGroupHost
	case strings.HasPrefix(name, "vcenter.resource_pool."):
		return ScraperGroupResourcePool
	case strings.HasPrefix(name, "vcenter.vm."):
		return ScraperGroupVM
	default:
		return ScraperGroupCluster
	}
}

// isScraperGroupEnabled returns whether a scraper group is enabled.
// If cfg.Scrapers is nil, all groups are enabled for backward compatibility reasons
func isScraperGroupEnabled(cfg *Config, group ScraperGroup) bool {
	if cfg.Scrapers == nil {
		return true
	}
	sc, ok := cfg.Scrapers[group]
	if !ok {
		return true
	}
	return sc.Enabled
}

// effectiveMetricsBuilderConfig returns a MetricsBuilderConfig with any metric disabled
// whose scraper group is disabled in cfg. Used so the metrics builder only emits metrics
// for enabled groups.
func effectiveMetricsBuilderConfig(cfg *Config, mbc metadata.MetricsBuilderConfig) metadata.MetricsBuilderConfig {
	if cfg.Scrapers == nil {
		return mbc
	}
	effective := mbc
	metricsVal := reflect.ValueOf(&effective.Metrics).Elem()
	metricsTyp := metricsVal.Type()
	for i := 0; i < metricsVal.NumField(); i++ {
		field := metricsVal.Field(i)
		if field.Kind() != reflect.Struct {
			continue
		}
		enabledField := field.FieldByName("Enabled")
		if !enabledField.IsValid() || !enabledField.CanSet() {
			continue
		}
		tag := metricsTyp.Field(i).Tag.Get("mapstructure")
		if tag == "" {
			continue
		}
		if !isScraperGroupEnabled(cfg, groupForMetricName(tag)) {
			enabledField.SetBool(false)
		}
	}
	return effective
}
