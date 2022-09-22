// Copyright 2020 OpenTelemetry Authors
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
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type snmpScraper struct {
	client   client
	logger   *zap.Logger
	cfg      *Config
	settings component.ReceiverCreateSettings
}

// newScraper creates an initialized snmpScraper
func newScraper(logger *zap.Logger, cfg *Config, settings component.ReceiverCreateSettings) *snmpScraper {
	return &snmpScraper{
		logger:   logger,
		cfg:      cfg,
		settings: settings,
	}
}

func (s *snmpScraper) start(ctx context.Context, host component.Host) (err error) {
	s.client, err = newClient(s.cfg, host, s.settings.TelemetrySettings, s.logger)
	if err != nil {
		return err
	}
	err = s.client.Connect()
	if err != nil {
		return err
	}

	return
}

func (s *snmpScraper) scrape(_ context.Context) (pmetric.Metrics, error) {
	md := pmetric.NewMetrics()

	scopeMetrics := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
	scopeMetrics.Scope().SetName("otelcol/snmpreceiver")
	scopeMetrics.Scope().SetVersion(s.settings.BuildInfo.Version)
	metricSlice := scopeMetrics.Metrics()
	now := pcommon.NewTimestampFromTime(time.Now())

	if err := s.scrapeScalarMetrics(now, &metricSlice); err != nil {
		return md, err
	}

	resourceMetricsMap := map[string]*pmetric.ResourceMetrics{}
	if err := s.scrapeIndexedMetrics(now, resourceMetricsMap); err != nil {
		return md, err
	}
	for _, value := range resourceMetricsMap {
		value.MoveTo(md.ResourceMetrics().AppendEmpty())
	}

	return md, nil
}

// scrapeScalarMetrics retrieves all SNMP data from scalar OIDs and turns them into metrics with attributes
func (s *snmpScraper) scrapeScalarMetrics(now pcommon.Timestamp, metricSlice *pmetric.MetricSlice) error {
	scalarMetricNamesByOID := map[string]string{}
	scalarMetricOIDs := []string{}

	// Find all metric scalar OIDs
	// Also create a map of metric names with OID as key so the metric will be easy to lookup later
	for name, metricCfg := range s.cfg.Metrics {
		if len(metricCfg.ScalarOIDs) > 0 {
			for i, oid := range metricCfg.ScalarOIDs {
				if !strings.HasPrefix(oid.OID, ".") {
					oid.OID = "." + oid.OID
					s.cfg.Metrics[name].ScalarOIDs[i].OID = oid.OID
				}
				scalarMetricOIDs = append(scalarMetricOIDs, oid.OID)
				scalarMetricNamesByOID[oid.OID] = name
			}
		}
	}

	if len(scalarMetricOIDs) == 0 {
		return nil
	}

	// Get all scalar OID data and turn it into metrics/attributes
	err := s.client.GetScalarData(scalarMetricOIDs, scalarDataToMetric(now, metricSlice, scalarMetricNamesByOID, s.cfg))
	if err != nil {
		return err
	}

	return nil
}

// scalarDataToMetric provides a function which will convert one piece of SNMP scalar data
// and turn it into a metric with attributes based on the config
func scalarDataToMetric(
	now pcommon.Timestamp,
	metricSlice *pmetric.MetricSlice,
	scalarMetricNamesByOID map[string]string,
	cfg *Config,
) processFunc {
	// returns a function because this is what the client GetScalarData method requires
	return func(data snmpData) error {
		switch data.valueType {
		case NotSupported:
			fallthrough
		case String:
			return fmt.Errorf("Returned metric data for OID: %s is not supported", data.oid)
		}

		// retrieve the metric config for this piece of SNMP data
		metricName := scalarMetricNamesByOID[data.oid]
		metricCfg := cfg.Metrics[metricName]

		// get all attributes for this metric config
		var metricAttributes []Attribute
		for _, scalarOID := range metricCfg.ScalarOIDs {
			if scalarOID.OID == data.oid {
				metricAttributes = scalarOID.Attributes
			}
		}

		// start building metric based on metric config
		builtMetric := metricSlice.AppendEmpty()
		builtMetric.SetName(metricName)
		builtMetric.SetDescription(metricCfg.Description)
		builtMetric.SetUnit(metricCfg.Unit)

		var dps pmetric.NumberDataPointSlice
		if metricCfg.Sum != nil {
			builtMetric.SetEmptySum()
			builtMetric.Sum().SetIsMonotonic(metricCfg.Sum.Monotonic)

			switch metricCfg.Sum.Aggregation {
			case "cumulative":
				builtMetric.Sum().SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
			case "delta":
				builtMetric.Sum().SetAggregationTemporality(pmetric.MetricAggregationTemporalityDelta)
			}
			dps = builtMetric.Sum().DataPoints()
		} else {
			builtMetric.SetEmptyGauge()
			dps = builtMetric.Gauge().DataPoints()
		}

		// creates a data point based on the SNMP data
		dp := dps.AppendEmpty()
		dp.SetTimestamp(now)
		switch data.valueType {
		case Integer:
			dp.SetIntVal(data.value.(int64))
		case Float:
			dp.SetDoubleVal(data.value.(float64))
		}

		// set enum attributes for this metric based on the previously gathered attributes
		// values will comes from these attributes and keys will come from the attribute config
		for _, attribute := range metricAttributes {
			attributeCfg := cfg.Attributes[attribute.Name]
			attributeKey := attribute.Name
			if attributeCfg.Value != "" {
				attributeKey = attributeCfg.Value
			}
			dp.Attributes().PutString(attributeKey, attribute.Value)
		}

		return nil
	}
}

// scrapeIndexedMetrics retrieves all SNMP data from column OIDs and turns them into metrics with attributes
func (s *snmpScraper) scrapeIndexedMetrics(now pcommon.Timestamp, resourceMetricsMap map[string]*pmetric.ResourceMetrics) error {
	// Retrieve column OID SNMP data for attributes
	indexedAttributeMapByOID := map[string]map[string]string{}
	s.scrapeIndexedAttributes(indexedAttributeMapByOID)

	// Retrieve column OID SNMP data for resource attributes
	indexedResourceAttributeMapByOID := map[string]map[string]string{}
	s.scrapeIndexedResourceAttributes(indexedResourceAttributeMapByOID)

	// Find all metric column OIDs
	// Also create a map of metric names with OID as key so the metric will be easy to lookup later
	indexedMetricNamesByOID := map[string]string{}
	indexedMetricOIDs := []string{}
	for name, metricCfg := range s.cfg.Metrics {
		if len(metricCfg.ColumnOIDs) > 0 {
			for i, oid := range metricCfg.ColumnOIDs {
				if !strings.HasPrefix(oid.OID, ".") {
					oid.OID = "." + oid.OID
					s.cfg.Metrics[name].ColumnOIDs[i].OID = oid.OID
				}
				indexedMetricOIDs = append(indexedMetricOIDs, oid.OID)
				indexedMetricNamesByOID[oid.OID] = name
			}
		}
	}

	if len(indexedMetricOIDs) == 0 {
		return nil
	}

	// Get all column OID data for metrics and turn it into metrics and attributes
	// (using the previously retrieved attribute data)
	err := s.client.GetIndexedData(
		indexedMetricOIDs,
		indexedDataToMetric(now, resourceMetricsMap, indexedMetricNamesByOID, indexedAttributeMapByOID, indexedResourceAttributeMapByOID, s),
	)
	if err != nil {
		return err
	}

	return nil
}

// indexedDataToMetric provides a function which will convert one piece of column OID SNMP indexed data
// and turn it into a metric with attributes based on the config and previously collected column OID
// SNMP indexed attribute data
func indexedDataToMetric(
	now pcommon.Timestamp,
	resourceMetricsMap map[string]*pmetric.ResourceMetrics,
	indexedMetricNamesByOID map[string]string,
	indexedAttributeMapByOID map[string]map[string]string,
	indexedResourceAttributeMapByOID map[string]map[string]string,
	snmpScraper *snmpScraper,
) processFunc {
	// returns a function because this is what the client GetIndexedData method requires
	return func(data snmpData) error {
		switch data.valueType {
		case NotSupported:
			fallthrough
		case String:
			return fmt.Errorf("Returned metric data for OID: %s is not supported", data.oid)
		}
		// retrieve the metric config for this piece of SNMP data
		cfg := snmpScraper.cfg
		metricName := indexedMetricNamesByOID[data.parentOID]
		metricCfg := cfg.Metrics[metricName]

		// get all attributes and resource attributes for this metric config
		var metricResourceAttributes []string
		var metricAttributes []Attribute
		for _, indexedOID := range metricCfg.ColumnOIDs {
			if indexedOID.OID == data.parentOID {
				metricAttributes = indexedOID.Attributes
				metricResourceAttributes = indexedOID.ResourceAttributes
			}
		}

		indexString := strings.TrimPrefix(data.oid, data.parentOID)
		resourceAttributes := map[string]string{}
		resourceAttributeNames := []string{}
		for _, attributeName := range metricResourceAttributes {
			resourceAttributeNames = append(resourceAttributeNames, attributeName)
			attributeCfg := cfg.ResourceAttributes[attributeName]

			var attributeValue string
			if attributeCfg.IndexedValuePrefix != "" {
				attributeValue = attributeCfg.IndexedValuePrefix + indexString
			} else if attributeCfg.OID != "" {
				attributeValue = indexedResourceAttributeMapByOID[attributeCfg.OID][indexString]
			}
			resourceAttributes[attributeName] = attributeValue
		}
		sort.Strings(resourceAttributeNames)
		resourceKey := ""
		if len(resourceAttributeNames) > 0 {
			resourceKey = strings.Join(resourceAttributeNames, "") + indexString
		}
		var metricSlice pmetric.MetricSlice
		if resourceMetricsMap[resourceKey] != nil {
			metricSlice = resourceMetricsMap[resourceKey].ScopeMetrics().At(0).Metrics()
		} else {
			resourceMetrics := pmetric.NewResourceMetrics()
			for key, value := range resourceAttributes {
				resourceMetrics.Resource().Attributes().UpsertString(key, value)
			}
			scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()
			scopeMetrics.Scope().SetName("otelcol/snmpreceiver")
			scopeMetrics.Scope().SetVersion(snmpScraper.settings.BuildInfo.Version)
			metricSlice = scopeMetrics.Metrics()
			resourceMetricsMap[resourceKey] = &resourceMetrics
		}

		// start building metric based on metric config
		builtMetric := metricSlice.AppendEmpty()
		builtMetric.SetName(metricName)
		builtMetric.SetDescription(metricCfg.Description)
		builtMetric.SetUnit(metricCfg.Unit)

		var dps pmetric.NumberDataPointSlice
		if metricCfg.Sum != nil {
			builtMetric.SetDataType(pmetric.MetricDataTypeSum)
			builtMetric.Sum().SetIsMonotonic(metricCfg.Sum.Monotonic)

			switch metricCfg.Sum.Aggregation {
			case "cumulative":
				builtMetric.Sum().SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
			case "delta":
				builtMetric.Sum().SetAggregationTemporality(pmetric.MetricAggregationTemporalityDelta)
			}
			dps = builtMetric.Sum().DataPoints()
		} else {
			builtMetric.SetDataType(pmetric.MetricDataTypeGauge)
			dps = builtMetric.Gauge().DataPoints()
		}

		// creates a data point based on the SNMP data
		dp := dps.AppendEmpty()
		dp.SetTimestamp(now)
		switch data.valueType {
		case Integer:
			dp.SetIntVal(data.value.(int64))
		case Float:
			dp.SetDoubleVal(data.value.(float64))
		}

		// set attributes for this metric based on the previously gathered attributes
		// keys will come from the attribute config and values will come from either the
		// attribute value if it was an enum attribute, SNMP data index and prefix value
		// in the config if it the prefix exists, or the map for indexed attribute SNMP
		// data if the config has an OID
		for _, attribute := range metricAttributes {
			attributeCfg := cfg.Attributes[attribute.Name]
			attributeKey := attribute.Name
			if attributeCfg.Value != "" {
				attributeKey = attributeCfg.Value
			}
			attributeValue := attribute.Value
			indexString := strings.TrimPrefix(data.oid, data.parentOID)
			if attributeCfg.IndexedValuePrefix != "" {
				attributeValue = attributeCfg.IndexedValuePrefix + indexString
			} else if attributeCfg.OID != "" {
				attributeValue = indexedAttributeMapByOID[attributeCfg.OID][indexString]
			}
			dp.Attributes().UpsertString(attributeKey, attributeValue)
		}

		return nil
	}
}

// scrapeIndexedAttributes retrieves SNMP data from attribute related column OIDs
func (s *snmpScraper) scrapeIndexedAttributes(indexedAttributeMapByOID map[string]map[string]string) error {
	// Find all attribute column OIDs
	indexedAttributeOIDs := []string{}
	for name, attributeCfg := range s.cfg.Attributes {
		if attributeCfg.OID != "" {
			if !strings.HasPrefix(attributeCfg.OID, ".") {
				attributeCfg.OID = "." + attributeCfg.OID
				s.cfg.Attributes[name] = attributeCfg
			}
			indexedAttributeOIDs = append(indexedAttributeOIDs, attributeCfg.OID)
		}
	}

	// Store column OID SNMP attribute data for later use
	err := s.client.GetIndexedData(
		indexedAttributeOIDs,
		indexedDataToAttribute(indexedAttributeMapByOID),
	)
	if err != nil {
		return err
	}

	return nil
}

// indexedDataToAttribute provides a function which will take one piece of column OID SNMP indexed data
// and store it in a map for later use.
// The key will be the OID of the attribute config, and the value with be another map
// The inner map's key will be the index of the SNMP data and the value will be the SNMP data value
// This way we can match indexed metrics to this data through the attribute config and the
// indices of the individual metrics
func indexedDataToAttribute(
	indexedAttributeMapByOID map[string]map[string]string,
) processFunc {
	// returns a function because this is what the client GetIndexedData method requires
	return func(data snmpData) error {
		var stringValue string
		switch data.valueType {
		case NotSupported:
			return fmt.Errorf("Returned attribute data for OID: %s from column OID: %s is not supported", data.oid, data.parentOID)
		case String:
			stringValue = data.value.(string)
		case Integer:
			stringValue = strconv.FormatInt(data.value.(int64), 10)
		case Float:
			stringValue = strconv.FormatFloat(data.value.(float64), 'f', 2, 64)
		}
		// For each piece of SNMP data related to an attribute column OID store it in a map
		indexString := strings.TrimPrefix(data.oid, data.parentOID)
		if indexedAttributeMapByOID[data.parentOID] == nil {
			indexedAttributeMapByOID[data.parentOID] = map[string]string{}
		}
		indexedAttributeMapByOID[data.parentOID][indexString] = stringValue

		return nil
	}
}

// scrapeIndexedResourceAttributes retrieves SNMP data from resource attribute related column OIDs
func (s *snmpScraper) scrapeIndexedResourceAttributes(indexedResourceAttributeMapByOID map[string]map[string]string) error {
	// Find all resource attribute column OIDs
	indexedResourceAttributeOIDs := []string{}
	for name, resourceAttributeCfg := range s.cfg.ResourceAttributes {
		if resourceAttributeCfg.OID != "" {
			if !strings.HasPrefix(resourceAttributeCfg.OID, ".") {
				resourceAttributeCfg.OID = "." + resourceAttributeCfg.OID
				s.cfg.ResourceAttributes[name] = resourceAttributeCfg
			}
			indexedResourceAttributeOIDs = append(indexedResourceAttributeOIDs, resourceAttributeCfg.OID)
		}
	}

	// Store column OID SNMP attribute data for later use
	err := s.client.GetIndexedData(
		indexedResourceAttributeOIDs,
		indexedDataToAttribute(indexedResourceAttributeMapByOID),
	)
	if err != nil {
		return err
	}

	return nil
}

func indexedDataToResourceAttributes(
	indexedResoiurceAttributeMapByOID map[string]map[string]string,
) processFunc {
	// returns a function because this is what the client GetIndexedData method requires
	return func(data snmpData) error {
		var stringValue string
		switch data.valueType {
		case NotSupported:
			return fmt.Errorf("Returned resource attribute data for OID: %s from column OID: %s is not supported", data.oid, data.parentOID)
		case String:
			stringValue = data.value.(string)
		case Integer:
			stringValue = strconv.FormatInt(data.value.(int64), 10)
		case Float:
			stringValue = strconv.FormatFloat(data.value.(float64), 'f', 2, 64)
		}
		// For each piece of SNMP data related to a resource attribute column OID store it in a map
		indexString := strings.TrimPrefix(data.oid, data.parentOID)
		if indexedResoiurceAttributeMapByOID[data.parentOID] == nil {
			indexedResoiurceAttributeMapByOID[data.parentOID] = map[string]string{}
		}
		indexedResoiurceAttributeMapByOID[data.parentOID][indexString] = stringValue

		return nil
	}
}
