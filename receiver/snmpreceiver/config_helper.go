// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package snmpreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver"

import (
	"strings"
)

// configHelper contains many of the functions required to get various info from the SNMP config
type configHelper struct {
	cfg                         *Config
	metricScalarOIDs            []string
	metricColumnOIDs            []string
	attributeColumnOIDs         []string
	resourceAttributeColumnOIDs []string
	metricNamesByOID            map[string]string
	metricAttributesByOID       map[string][]Attribute
	resourceAttributesByOID     map[string][]string
}

// newConfigHelper returns a new configHelper with various pieces of static info saved for easy access
func newConfigHelper(cfg *Config) *configHelper {
	ch := configHelper{
		cfg:                         cfg,
		metricScalarOIDs:            []string{},
		metricColumnOIDs:            []string{},
		attributeColumnOIDs:         []string{},
		resourceAttributeColumnOIDs: []string{},
		metricNamesByOID:            map[string]string{},
		metricAttributesByOID:       map[string][]Attribute{},
		resourceAttributesByOID:     map[string][]string{},
	}

	// Group all metric scalar OIDs and metric column OIDs
	// Also create a map of metric names with OID as key so the metric config will be easy to
	// matchup later with returned SNMP data
	for name, metricCfg := range cfg.Metrics {
		for i, oid := range metricCfg.ScalarOIDs {
			// Data is returned by the client with '.' prefix on the OIDs.
			// Making sure the prefix exists here in the configs so we can match it up with returned data later
			if !strings.HasPrefix(oid.OID, ".") {
				oid.OID = "." + oid.OID
				cfg.Metrics[name].ScalarOIDs[i].OID = oid.OID
			}
			ch.metricScalarOIDs = append(ch.metricScalarOIDs, oid.OID)
			ch.metricNamesByOID[oid.OID] = name
			ch.metricAttributesByOID[oid.OID] = oid.Attributes
		}

		for i, oid := range metricCfg.ColumnOIDs {
			// Data is returned by the client with '.' prefix on the OIDs.
			// Making sure the prefix exists here in the configs so we can match it up with returned data later
			if !strings.HasPrefix(oid.OID, ".") {
				oid.OID = "." + oid.OID
				cfg.Metrics[name].ColumnOIDs[i].OID = oid.OID
			}
			ch.metricColumnOIDs = append(ch.metricColumnOIDs, oid.OID)
			ch.metricNamesByOID[oid.OID] = name
			ch.metricAttributesByOID[oid.OID] = oid.Attributes
			ch.resourceAttributesByOID[oid.OID] = oid.ResourceAttributes
		}
	}

	// Find all attribute column OIDs
	for name, attributeCfg := range cfg.Attributes {
		if attributeCfg.OID == "" {
			continue
		}

		// Data is returned by the client with '.' prefix on the OIDs.
		// Making sure the prefix exists here in the configs so we can match it up with returned data later
		if !strings.HasPrefix(attributeCfg.OID, ".") {
			attributeCfg.OID = "." + attributeCfg.OID
			cfg.Attributes[name] = attributeCfg
		}
		ch.attributeColumnOIDs = append(ch.attributeColumnOIDs, attributeCfg.OID)
	}

	// Find all resource attribute column OIDs
	for name, resourceAttributeCfg := range cfg.ResourceAttributes {
		if resourceAttributeCfg.OID == "" {
			continue
		}

		// Data is returned by the client with '.' prefix on the OIDs.
		// Making sure the prefix exists here in the configs so we can match it up with returned data later
		if !strings.HasPrefix(resourceAttributeCfg.OID, ".") {
			resourceAttributeCfg.OID = "." + resourceAttributeCfg.OID
			cfg.ResourceAttributes[name] = resourceAttributeCfg
		}
		ch.resourceAttributeColumnOIDs = append(ch.resourceAttributeColumnOIDs, resourceAttributeCfg.OID)
	}

	return &ch
}

// getMetricScalarOIDs returns all of the scalar OIDs in the metric configs
func (h configHelper) getMetricScalarOIDs() []string {
	return h.metricScalarOIDs
}

// getMetricColumnOIDs returns all of the column OIDs in the metric configs
func (h configHelper) getMetricColumnOIDs() []string {
	return h.metricColumnOIDs
}

// getAttributeColumnOIDs returns all of the attribute column OIDs in the attribute configs
func (h configHelper) getAttributeColumnOIDs() []string {
	return h.attributeColumnOIDs
}

// getResourceAttributeColumnOIDs returns all of the resource attribute column OIDs in the resource attribute configs
func (h configHelper) getResourceAttributeColumnOIDs() []string {
	return h.resourceAttributeColumnOIDs
}

// getMetricName a metric names based on a given OID
func (h configHelper) getMetricName(oid string) string {
	return h.metricNamesByOID[oid]
}

// getMetricConfig returns a metric config based on a given name
func (h configHelper) getMetricConfig(name string) *MetricConfig {
	return h.cfg.Metrics[name]
}

// getAttributeConfigValue returns the value of an attribute config
func (h configHelper) getAttributeConfigValue(name string) string {
	attrConfig := h.cfg.Attributes[name]
	if attrConfig == nil {
		return ""
	}

	return attrConfig.Value
}

// getAttributeConfigIndexedValuePrefix returns the indexed value prefix of an attribute config
func (h configHelper) getAttributeConfigIndexedValuePrefix(name string) string {
	attrConfig := h.cfg.Attributes[name]
	if attrConfig == nil {
		return ""
	}

	return attrConfig.IndexedValuePrefix
}

// getAttributeConfigOID returns the column OID of an attribute config
func (h configHelper) getAttributeConfigOID(name string) string {
	attrConfig := h.cfg.Attributes[name]
	if attrConfig == nil {
		return ""
	}

	return attrConfig.OID
}

// getResourceAttributeConfigIndexedValuePrefix returns the indexed value prefix of a resource attribute config
func (h configHelper) getResourceAttributeConfigIndexedValuePrefix(name string) string {
	attrConfig := h.cfg.ResourceAttributes[name]
	if attrConfig == nil {
		return ""
	}

	return attrConfig.IndexedValuePrefix
}

// getResourceAttributeConfigOID returns the column OID of a resource attribute config
func (h configHelper) getResourceAttributeConfigOID(name string) string {
	attrConfig := h.cfg.ResourceAttributes[name]
	if attrConfig == nil {
		return ""
	}

	return attrConfig.OID
}

// getMetricConfigAttributes returns the metric config attributes for a given OID
func (h configHelper) getMetricConfigAttributes(oid string) []Attribute {
	return h.metricAttributesByOID[oid]
}

// getResourceAttributeNames returns the metric config resource attributes for a given OID
func (h configHelper) getResourceAttributeNames(oid string) []string {
	return h.resourceAttributesByOID[oid]
}
