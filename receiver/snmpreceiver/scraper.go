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

	"github.com/go-errors/errors"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

const generalResourceKey = ""

var (
	// Errors
	errScrape = errors.New("failed to successfully scrape any SNMP metrics")

	// Error messages
	errMsgBadValueType                  = `returned metric SNMP data type for OID: %s is not supported`
	errMsgIndexedBadValueType           = `returned metric SNMP data type for OID: %s from column OID: %s is not supported`
	errMsgBadIndexedAttributes          = `problem retrieving SNMP indexed attribute data: %w`
	errMsgBadIndexedResourceAttributes  = `problem retrieving SNMP indexed resource attribute data: %w`
	errMsgIndexedAttributesBadValueType = `returned attribute SNMP data type for OID: %s from column OID: %s is not supported`
)

// snmpScraper handles scraping of SNMP metrics
type snmpScraper struct {
	client   client
	logger   *zap.Logger
	cfg      *Config
	settings component.ReceiverCreateSettings
}

// metricsByResourceContainer is a helper struct to hold metric data keyed by resource key and metric name
type metricsByResourceContainer struct {
	metricsByResource map[string]map[string]*pmetric.Metric
}

func newMetricsByResourceContainer() *metricsByResourceContainer {
	return &metricsByResourceContainer{
		metricsByResource: map[string]map[string]*pmetric.Metric{},
	}
}

func (m *metricsByResourceContainer) getMetric(resourceKey string, metricName string) *pmetric.Metric {
	return m.metricsByResource[resourceKey][metricName]
}

func (m *metricsByResourceContainer) putMetric(resourceKey string, metricName string, metric *pmetric.Metric) {
	if m.metricsByResource[resourceKey] == nil {
		m.metricsByResource[resourceKey] = map[string]*pmetric.Metric{}
	}
	m.metricsByResource[resourceKey][metricName] = metric
}

func (m *metricsByResourceContainer) putMetrics(resourceKey string, metrics map[string]*pmetric.Metric) {
	m.metricsByResource[resourceKey] = metrics
}

func (m *metricsByResourceContainer) metricCountByResource(resourceKey string) int {
	return len(m.metricsByResource[resourceKey])
}

func (m *metricsByResourceContainer) resourceCount() int {
	return len(m.metricsByResource)
}

// indexedAttributeKey is a complex key for attribute value maps
type indexedAttributeKey struct {
	parentOID string
	oidIndex  string
}

// newScraper creates an initialized snmpScraper
func newScraper(logger *zap.Logger, cfg *Config, settings component.ReceiverCreateSettings) *snmpScraper {
	return &snmpScraper{
		logger:   logger,
		cfg:      cfg,
		settings: settings,
	}
}

// start gets the client ready
func (s *snmpScraper) start(_ context.Context, host component.Host) (err error) {
	s.client, err = newClient(s.cfg, host, s.settings.TelemetrySettings, s.logger)
	if err != nil {
		return err
	}

	return s.client.Connect()
}

// shutdown closes the client
func (s *snmpScraper) shutdown(_ context.Context, _ component.Host) error {
	return s.client.Close()
}

// scrape collects and creates OTEL metrics from a SNMP environment
func (s *snmpScraper) scrape(_ context.Context) (pmetric.Metrics, error) {
	// Get a basic ResourceMetrics prepped for metrics with no resource attributes
	resourceMetrics := pmetric.NewResourceMetrics()
	scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()
	scopeMetrics.Scope().SetName("otelcol/snmpreceiver")
	scopeMetrics.Scope().SetVersion(s.settings.BuildInfo.Version)
	metricSlice := scopeMetrics.Metrics()
	now := pcommon.NewTimestampFromTime(time.Now())

	metricsProcessed := false

	resourceMetricsMap := map[string]*pmetric.ResourceMetrics{}
	metricsMap := map[string]*pmetric.Metric{}
	// Try to scrape scalar OID based metrics.
	// metricsMap is passed in as a place for any created metrics to live
	if err := s.scrapeScalarMetrics(now, metricsMap); err != nil {
		s.logger.Warn("Failed to collect scalar OID metrics", zap.Error(err))
	} else if len(metricsMap) != 0 {
		metricsProcessed = true
		// Add new scalar metrics to the basic ResourceMetrics with no resource attributes
		for _, value := range metricsMap {
			value.MoveTo(metricSlice.AppendEmpty())
		}
		// Load the map with the basic ResourceMetrics (that possibly
		// has scalar OID based metrics already on it)
		resourceMetricsMap[""] = &resourceMetrics
	}

	// Load this with the scalar metric map that was just created for the
	// basic ResourceMetrics. The info in this map is ultimately already
	// contained within the resourceMetricsMap, but it is more easily accessible
	// to pull out a specific existing Metric by resource and metric name using this
	metricsByResource := newMetricsByResourceContainer()
	metricsByResource.putMetrics(generalResourceKey, metricsMap)

	// Try to scrape column OID based metrics.
	// resourceMetricsMap is passed in as a place for any created resources/metrics to live
	// metricsByResource is provided as an easy way to check if a metric is already
	// associated with an existing resource
	scalarMetricCnt := len(metricsMap)
	if err := s.scrapeIndexedMetrics(now, resourceMetricsMap, metricsByResource); err != nil {
		s.logger.Warn("Failed to collect column OID metrics", zap.Error(err))
	} else if metricsByResource.metricCountByResource(generalResourceKey) > scalarMetricCnt || metricsByResource.resourceCount() > 1 {
		metricsProcessed = true
	}

	// Return error if we failed to scrape any metrics
	md := pmetric.NewMetrics()
	if !metricsProcessed {
		return md, errScrape
	}

	// Put all of the created ResourceMetrics onto a top level Metrics
	for _, value := range resourceMetricsMap {
		value.MoveTo(md.ResourceMetrics().AppendEmpty())
	}

	return md, nil
}

// scrapeScalarMetrics retrieves all SNMP data from scalar OIDs and turns the returned scalar data
// into metrics with optional enum attributes
func (s *snmpScraper) scrapeScalarMetrics(now pcommon.Timestamp, metricsMap map[string]*pmetric.Metric) error {
	scalarMetricNamesByOID := map[string]string{}
	scalarMetricOIDs := []string{}

	// Find all metric scalar OIDs
	// Also create a map of metric names with OID as key so the metric config will be easy to
	// matchup later with returned data
	for name, metricCfg := range s.cfg.Metrics {
		if len(metricCfg.ScalarOIDs) > 0 {
			for i, oid := range metricCfg.ScalarOIDs {
				// Data is returned by the client with '.' prefix on the OIDs.
				// Making sure the prefix exists here in the configs so we can match it up with returned data later
				if !strings.HasPrefix(oid.OID, ".") {
					oid.OID = "." + oid.OID
					s.cfg.Metrics[name].ScalarOIDs[i].OID = oid.OID
				}
				scalarMetricOIDs = append(scalarMetricOIDs, oid.OID)
				scalarMetricNamesByOID[oid.OID] = name
			}
		}
	}

	// If no scalar metric configs, nothing else to do
	if len(scalarMetricOIDs) == 0 {
		return nil
	}

	// Get all SNMP scalar OID data and turn it into metrics/attributes
	// which are then stored in the passed in metricsMap
	return s.client.GetScalarData(scalarMetricOIDs, scalarDataToMetric(now, metricsMap, scalarMetricNamesByOID, s.cfg))
}

// scalarDataToMetric provides a function which will convert one piece of SNMP scalar data, turn it into
// a metric with attributes based on the related configs, store it in the passed in metricsMap
func scalarDataToMetric(
	now pcommon.Timestamp,
	metricsMap map[string]*pmetric.Metric,
	scalarMetricNamesByOID map[string]string,
	cfg *Config,
) processFunc {
	// This returns a processFunc because this is what the client's GetScalarData method requires
	return func(data snmpData) error {
		// Return an error if this SNMP scalar data is not of a useable type
		if data.valueType == notSupportedVal || data.valueType == stringVal {
			return fmt.Errorf(errMsgBadValueType, data.oid)
		}

		// Retrieve the metric config for this SNMP data
		metricName := scalarMetricNamesByOID[data.oid]
		metricCfg := cfg.Metrics[metricName]

		// Get all enum attributes names and values for this SNMP scalar data based on the metric config
		var metricAttributes []Attribute
		for _, scalarOID := range metricCfg.ScalarOIDs {
			if scalarOID.OID == data.oid {
				metricAttributes = scalarOID.Attributes
			}
		}

		// Get/create the metric and datapoint for this SNMP scalar data and make sure metricsMap is current
		metric := metricsMap[metricName]
		metric, dp := createNewMetricDataPoint(now, data, metric, metricName, metricCfg)
		if metricsMap[metricName] == nil {
			metricsMap[metricName] = metric
		}

		// Set enum attributes for this metric's datapoint based on the previously gathered attributes.
		// Keys will be determined from the related attribute config and enum values will come straight from
		// the metric config's attributes.
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

// scrapeIndexedMetrics retrieves all SNMP data from column OIDs and turns the returned indexed data
// into metrics with optional attribute and/or resource attributes
func (s *snmpScraper) scrapeIndexedMetrics(
	now pcommon.Timestamp,
	resourceMetricsMap map[string]*pmetric.ResourceMetrics,
	metricsByResource *metricsByResourceContainer,
) error {
	// Retrieve column OID SNMP indexed data for attributes
	indexedAttributeValues := map[indexedAttributeKey]string{}
	err := s.scrapeIndexedAttributes(indexedAttributeValues)
	if err != nil {
		return fmt.Errorf(errMsgBadIndexedAttributes, err)
	}

	// Retrieve column OID SNMP indexed data for resource attributes
	indexedResourceAttributeValues := map[indexedAttributeKey]string{}
	err = s.scrapeIndexedResourceAttributes(indexedResourceAttributeValues)
	if err != nil {
		return fmt.Errorf(errMsgBadIndexedResourceAttributes, err)
	}

	// Find all metric column OIDs
	// Also create a map of metric names with OID as key so the metric config will be easy to
	// matchup later with returned SNMP indexed data
	indexedMetricNamesByOID := map[string]string{}
	indexedMetricOIDs := []string{}
	for name, metricCfg := range s.cfg.Metrics {
		if len(metricCfg.ColumnOIDs) > 0 {
			for i, oid := range metricCfg.ColumnOIDs {
				// Data is returned by the client with '.' prefix on the OIDs.
				// Making sure the prefix exists here in the configs so we can match it up with returned data later
				if !strings.HasPrefix(oid.OID, ".") {
					oid.OID = "." + oid.OID
					s.cfg.Metrics[name].ColumnOIDs[i].OID = oid.OID
				}
				indexedMetricOIDs = append(indexedMetricOIDs, oid.OID)
				indexedMetricNamesByOID[oid.OID] = name
			}
		}
	}

	// If no column metric configs, nothing else to do
	if len(indexedMetricOIDs) == 0 {
		return nil
	}

	// Get all column OID SNMP indexed data for metrics and turn it into metrics, attributes,
	// and resource attributes (using the previously retrieved attribute and resource attribute data)
	return s.client.GetIndexedData(
		indexedMetricOIDs,
		indexedDataToMetric(
			now, resourceMetricsMap, metricsByResource, indexedMetricNamesByOID,
			indexedAttributeValues, indexedResourceAttributeValues, s,
		),
	)
}

// indexedDataToMetric provides a function which will convert one piece of column OID SNMP indexed data
// and turn it into a metric with attributes based on the config and previously collected column OID
// SNMP indexed attribute. A resource may also be created if none exists for the related previously
// collected resource attribute data yet.
func indexedDataToMetric(
	now pcommon.Timestamp,
	resourceMetricsMap map[string]*pmetric.ResourceMetrics,
	metricsByResource *metricsByResourceContainer,
	indexedMetricNamesByOID map[string]string,
	indexedAttributeValues map[indexedAttributeKey]string,
	indexedResourceAttributeValues map[indexedAttributeKey]string,
	snmpScraper *snmpScraper,
) processFunc {
	// This returns a processFunc because this is what the client's GetScalarData method requires
	return func(data snmpData) error {
		// Return an error if this SNMP scalar data is not of a useable type
		switch data.valueType {
		case notSupportedVal:
			fallthrough
		case stringVal:
			return fmt.Errorf(errMsgIndexedBadValueType, data.oid, data.parentOID)
		}

		// Retrieve the metric config for this SNMP data
		cfg := snmpScraper.cfg
		metricName := indexedMetricNamesByOID[data.parentOID]
		metricCfg := cfg.Metrics[metricName]

		// Get all related attribute and resource attribute info for this SNMP indexed data based on the metric config
		var metricResourceAttributes []string
		var metricAttributes []Attribute
		for _, columnOID := range metricCfg.ColumnOIDs {
			if columnOID.OID == data.parentOID {
				metricAttributes = columnOID.Attributes
				metricResourceAttributes = columnOID.ResourceAttributes
			}
		}

		indexString := strings.TrimPrefix(data.oid, data.parentOID)
		resourceAttributes := map[string]string{}
		resourceAttributeNames := []string{}
		// Create a map of key/values for all related resource attributes. Keys will come directly from the
		// metric config's resource attribute values. Values will come from the related attribute config's
		// prefix value plus the index OR the previously collected resource attribute indexed data.
		for _, attributeName := range metricResourceAttributes {
			resourceAttributeNames = append(resourceAttributeNames, attributeName)
			resourceAttributeCfg := cfg.ResourceAttributes[attributeName]

			var attributeValue string
			if resourceAttributeCfg.IndexedValuePrefix != "" {
				attributeValue = resourceAttributeCfg.IndexedValuePrefix + indexString
			} else if resourceAttributeCfg.OID != "" {
				attrKey := indexedAttributeKey{
					parentOID: resourceAttributeCfg.OID,
					oidIndex:  indexString,
				}
				attributeValue = indexedResourceAttributeValues[attrKey]
			}
			resourceAttributes[attributeName] = attributeValue
		}
		// Create a resource key using all of the relevant resource attribute names
		sort.Strings(resourceAttributeNames)
		resourceKey := ""
		if len(resourceAttributeNames) > 0 {
			resourceKey = strings.Join(resourceAttributeNames, "...") + indexString
		}
		// If this resource exists, then grab the relevant metric slice to put new metric data on
		// If it doesn't, then create a new ResourceMetrics with the resource metric map that we
		// just created and use the newly created metric slice for new metric data
		var metricSlice pmetric.MetricSlice
		if resourceMetricsMap[resourceKey] != nil {
			metricSlice = resourceMetricsMap[resourceKey].ScopeMetrics().At(0).Metrics()
		} else {
			resourceMetrics := pmetric.NewResourceMetrics()
			for key, value := range resourceAttributes {
				resourceMetrics.Resource().Attributes().PutString(key, value)
			}
			scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()
			scopeMetrics.Scope().SetName("otelcol/snmpreceiver")
			scopeMetrics.Scope().SetVersion(snmpScraper.settings.BuildInfo.Version)
			metricSlice = scopeMetrics.Metrics()
			resourceMetricsMap[resourceKey] = &resourceMetrics
		}

		// Get/create the metric and datapoint for this SNMP data, make sure metricsMap is current,
		// and assign the metric to the correct resource if it is new
		metric := metricsByResource.getMetric(resourceKey, metricName)
		metric, dp := createNewMetricDataPoint(now, data, metric, metricName, metricCfg)
		if metricsByResource.getMetric(resourceKey, metricName) == nil {
			resourceMetric := metricSlice.AppendEmpty()
			metric.MoveTo(resourceMetric)
			metricsByResource.putMetric(resourceKey, metricName, &resourceMetric)
		}

		// Set attributes for this metric's datapoint based on the previously gathered attributes.
		// Keys will be determined from the related attribute config and values will come a few
		// different places.
		// Enum attribute value - comes from the metric config's attribute data
		// Indexed prefix attribute value - comes from the current SNMP data's index and the attribute
		// config's prefix value
		// Indexed OID attribute value - comes from the previously collected indexed attribute data
		// using the current index and attribute config to access the correct value
		for _, attribute := range metricAttributes {
			attributeCfg := cfg.Attributes[attribute.Name]
			attributeKey := attribute.Name
			if attributeCfg.Value != "" {
				attributeKey = attributeCfg.Value
			}
			attributeValue := attribute.Value
			if attributeCfg.IndexedValuePrefix != "" {
				attributeValue = attributeCfg.IndexedValuePrefix + indexString
			} else if attributeCfg.OID != "" {
				attrKey := indexedAttributeKey{
					parentOID: attributeCfg.OID,
					oidIndex:  indexString,
				}
				attributeValue = indexedAttributeValues[attrKey]
			}
			dp.Attributes().PutString(attributeKey, attributeValue)
		}

		return nil
	}
}

// createNewMetricDataPoint creates a new datapoint using SNMP data and a given metric.
// If the given metric doesn't exist, this is also created
func createNewMetricDataPoint(
	now pcommon.Timestamp,
	data snmpData,
	metric *pmetric.Metric,
	metricName string,
	metricCfg *MetricConfig,
) (*pmetric.Metric, *pmetric.NumberDataPoint) {
	// Either use a previously created metric or create a brand new metric.
	// This is so we don't create new metrics when the only things that has
	// changed is an attribute value
	var goodMetric pmetric.Metric
	var dps pmetric.NumberDataPointSlice
	if metric != nil {
		goodMetric = *metric
		if metricCfg.Sum != nil {
			dps = goodMetric.Sum().DataPoints()
		} else {
			dps = goodMetric.Gauge().DataPoints()
		}
	} else {
		goodMetric = pmetric.NewMetric()
		goodMetric.SetName(metricName)
		goodMetric.SetDescription(metricCfg.Description)
		goodMetric.SetUnit(metricCfg.Unit)

		if metricCfg.Sum != nil {
			goodMetric.SetEmptySum()
			goodMetric.Sum().SetIsMonotonic(metricCfg.Sum.Monotonic)

			switch metricCfg.Sum.Aggregation {
			case "cumulative":
				goodMetric.Sum().SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
			case "delta":
				goodMetric.Sum().SetAggregationTemporality(pmetric.MetricAggregationTemporalityDelta)
			}
			dps = goodMetric.Sum().DataPoints()
		} else {
			goodMetric.SetEmptyGauge()
			dps = goodMetric.Gauge().DataPoints()
		}
	}

	// Creates a data point based on the SNMP data
	dp := dps.AppendEmpty()
	dp.SetTimestamp(now)
	// Not explicitly checking these casts as this should be made safe in the client
	switch data.valueType {
	case integerVal:
		dp.SetIntValue(data.value.(int64))
	case floatVal:
		dp.SetDoubleValue(data.value.(float64))
	}

	return &goodMetric, &dp
}

// scrapeIndexedAttributes retrieves all SNMP data from attribute config column OIDs and
// stores the returned indexed data for later use by metrics
func (s *snmpScraper) scrapeIndexedAttributes(indexedAttributeValues map[indexedAttributeKey]string) error {
	// Find all attribute column OIDs
	indexedAttributeOIDs := []string{}
	for name, attributeCfg := range s.cfg.Attributes {
		if attributeCfg.OID != "" {
			// Data is returned by the client with '.' prefix on the OIDs.
			// Making sure the prefix exists here in the configs so we can match it up with returned data later
			if !strings.HasPrefix(attributeCfg.OID, ".") {
				attributeCfg.OID = "." + attributeCfg.OID
				s.cfg.Attributes[name] = attributeCfg
			}
			indexedAttributeOIDs = append(indexedAttributeOIDs, attributeCfg.OID)
		}
	}

	// If no oid attribute configs, nothing else to do
	if len(indexedAttributeOIDs) == 0 {
		return nil
	}

	// Retrieve SNMP attribute indexed data and store for later use
	return s.client.GetIndexedData(
		indexedAttributeOIDs,
		indexedDataToAttribute(indexedAttributeValues),
	)
}

// scrapeIndexedResourceAttributes retrieves all SNMP data from resource attribute config column OIDs and
// stores the returned indexed data for later use by metrics
func (s *snmpScraper) scrapeIndexedResourceAttributes(indexedResourceAttributeValues map[indexedAttributeKey]string) error {
	// Find all resource attribute column OIDs
	indexedResourceAttributeOIDs := []string{}
	for name, resourceAttributeCfg := range s.cfg.ResourceAttributes {
		if resourceAttributeCfg.OID != "" {
			// Data is returned by the client with '.' prefix on the OIDs.
			// Making sure the prefix exists here in the configs so we can match it up with returned data later
			if !strings.HasPrefix(resourceAttributeCfg.OID, ".") {
				resourceAttributeCfg.OID = "." + resourceAttributeCfg.OID
				s.cfg.ResourceAttributes[name] = resourceAttributeCfg
			}
			indexedResourceAttributeOIDs = append(indexedResourceAttributeOIDs, resourceAttributeCfg.OID)
		}
	}

	// If no OID resource attribute configs, nothing else to do
	if len(indexedResourceAttributeOIDs) == 0 {
		return nil
	}

	// Retrieve resource attribute indexed data and store for later use
	return s.client.GetIndexedData(
		indexedResourceAttributeOIDs,
		indexedDataToAttribute(indexedResourceAttributeValues),
	)
}

// indexedDataToAttribute provides a function which will take one piece of column OID SNMP indexed data
// (for either an attribute or resource attribute) and stores it in a map for later use (keyed by both
// {resource} attribute config column OID and OID index)
func indexedDataToAttribute(
	indexedAttributeValues map[indexedAttributeKey]string,
) processFunc {
	// This returns a processFunc because this is what the client's GetIndexedData method requires
	return func(data snmpData) error {
		// Get the string value of the SNMP data for the {resource} attribute value
		var stringValue string
		// Not explicitly checking these casts as this should be made safe in the client
		switch data.valueType {
		case notSupportedVal:
			return fmt.Errorf(errMsgIndexedAttributesBadValueType, data.oid, data.parentOID)
		case stringVal:
			stringValue = data.value.(string)
		case integerVal:
			stringValue = strconv.FormatInt(data.value.(int64), 10)
		case floatVal:
			stringValue = strconv.FormatFloat(data.value.(float64), 'f', 2, 64)
		}
		// Store the {resource} attribute value in a map using the column OID and OID index associated
		// as keys. This way we can match indexed metrics to this data through the {resource} attribute
		// config and the indices of the individual metric values
		indexString := strings.TrimPrefix(data.oid, data.parentOID)
		attrKey := indexedAttributeKey{
			parentOID: data.parentOID,
			oidIndex:  indexString,
		}
		indexedAttributeValues[attrKey] = stringValue

		return nil
	}
}
