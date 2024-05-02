// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter"

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/confmap"
)

var _ error = (*deprecatedError)(nil)

// deprecatedError is an error related to a renamed setting.
type deprecatedError struct {
	// oldName of the configuration option.
	oldName string
	// newName of the configuration option.
	newName string
	// updateFn updates the configuration to map the old value into the new one.
	// It must only be called when the old value is set and is not the default.
	updateFn func(*Config)
}

// List of settings that are deprecated but not yet removed.
var renamedSettings = []deprecatedError{
	{
		oldName: "metrics::histograms::send_count_sum_metrics",
		newName: "metrics::histograms::send_aggregation_metrics",
		updateFn: func(c *Config) {
			c.Metrics.HistConfig.SendAggregations = c.Metrics.HistConfig.SendCountSum
		},
	},
	{
		oldName: "traces::peer_service_aggregation",
		newName: "traces::peer_tags_aggregation",
		updateFn: func(c *Config) {
			c.Traces.PeerTagsAggregation = c.Traces.PeerServiceAggregation
		},
	},
}

// Error implements the error interface.
func (e deprecatedError) Error() string {
	return fmt.Sprintf(
		"%q has been deprecated in favor of %q",
		e.oldName,
		e.newName,
	)
}

// Check if the deprecated option is being used.
// Error out if both the old and new options are being used.
func (e deprecatedError) Check(configMap *confmap.Conf) (bool, error) {
	if configMap.IsSet(e.oldName) && configMap.IsSet(e.newName) {
		return false, fmt.Errorf("%q and %q can't be both set at the same time: use %q only instead", e.oldName, e.newName, e.newName)
	}
	return configMap.IsSet(e.oldName), nil
}

// UpdateCfg to move the old configuration value into the new one.
func (e deprecatedError) UpdateCfg(cfg *Config) {
	e.updateFn(cfg)
}

// handleRenamedSettings for a given configuration map.
// Error out if any pair of old-new options are set at the same time.
func handleRenamedSettings(configMap *confmap.Conf, cfg *Config) (warnings []error, err error) {
	var errs []error
	for _, renaming := range renamedSettings {
		isOldNameUsed, errCheck := renaming.Check(configMap)
		errs = append(errs, errCheck)

		if errCheck == nil && isOldNameUsed {
			warnings = append(warnings, renaming)
			// only update config if old name is in use
			renaming.UpdateCfg(cfg)
		}
	}
	err = errors.Join(errs...)

	return
}
