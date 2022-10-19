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
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/zap"
)

const generalResourceKey = ""

var (
	// Error messages
	errMsgBadValueType                          = `returned metric SNMP data type for OID '%s' is not supported`
	errMsgIndexedBadValueType                   = `returned metric SNMP data type for OID '%s' from column OID '%s' is not supported`
	errMsgIndexedAttributesBadValueType         = `returned attribute SNMP data type for OID '%s' from column OID '%s' is not supported`
	errMsgAttributeEmptyValue                   = `metric OID attribute value is blank`
	errMsgOIDAttributeEmptyValue                = `not creating indexed metric '%s' datapoint: %w`
	errMsgResourceAttributeEmptyValue           = `related resource attribute value is blank`
	errMsgOIDResourceAttributeEmptyValue        = `not creating indexed metric '%s' or resource: %w`
	errMsgScalarOIDProcessing                   = `problem processing scalar metric data for OID '%s': %w`
	errMsgIndexedMetricOIDProcessing            = `problem processing indexed metric data for OID '%s' from column OID '%s': %w`
	errMsgIndexedAttributeOIDProcessing         = `problem processing indexed attribute data for OID '%s' from column OID '%s': %w`
	errMsgIndexedResourceAttributeOIDProcessing = `problem processing indexed resource attribute data for OID '%s' from column OID '%s': %w`
)

// snmpScraper handles scraping of SNMP metrics
type snmpScraper struct {
	client   client
	logger   *zap.Logger
	cfg      *Config
	settings component.ReceiverCreateSettings
}

// metricsByResourceContainer is a helper alias to hold metric data keyed by resource key and metric name
type metricsByResourceContainer map[string]map[string]*pmetric.Metric

func newMetricsByResourceContainer() metricsByResourceContainer {
	mbrc := metricsByResourceContainer{}

	return mbrc
}

func (m metricsByResourceContainer) getMetrics(resourceKey string) map[string]*pmetric.Metric {
	if m[resourceKey] == nil {
		m[resourceKey] = map[string]*pmetric.Metric{}
	}
	return m[resourceKey]
}

// snmpOTELMetricData is a helper data struct to contain OTEL related data that will be used by many functions
type snmpOTELMetricData struct {
	// This is used as an easy reference to grab existing OTEL resources by unique key
	resourcesByKey map[string]*pmetric.ResourceMetrics
	// The info in this map will ultimately already be contained within the resourcesByKey, but it is
	// more easily accessible to pull out a specific existing Metric by resource and metric name using this
	metricsByResource metricsByResourceContainer
	// This is the ResourceMetricsSlice that will contain all newly created resources and metrics
	resourceMetricsSlice *pmetric.ResourceMetricsSlice
}

func newSNMPOTELMetricData(resourceMetricsSlice *pmetric.ResourceMetricsSlice) *snmpOTELMetricData {
	otelMetricData := snmpOTELMetricData{
		resourceMetricsSlice: resourceMetricsSlice,
		resourcesByKey:       map[string]*pmetric.ResourceMetrics{},
		metricsByResource:    newMetricsByResourceContainer(),
	}

	return &otelMetricData
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
	s.client, err = newClient(s.cfg, s.logger)

	return err
}

// scrape collects and creates OTEL metrics from a SNMP environment
func (s *snmpScraper) scrape(_ context.Context) (pmetric.Metrics, error) {
	if err := s.client.Connect(); err != nil {
		return pmetric.NewMetrics(), fmt.Errorf("problem connecting to SNMP host: %w", err)
	}
	defer s.client.Close()

	// Get a Metrics prepped all of the resources/metrics that we will be creating
	metrics := pmetric.NewMetrics()
	resourceMetricsSlice := metrics.ResourceMetrics()
	now := pcommon.NewTimestampFromTime(time.Now())

	otelMetricData := newSNMPOTELMetricData(&resourceMetricsSlice)

	var scraperErrors scrapererror.ScrapeErrors
	// Try to scrape scalar OID based metrics
	s.scrapeScalarMetrics(now, otelMetricData, &scraperErrors)

	// Try to scrape column OID based metrics
	s.scrapeIndexedMetrics(now, otelMetricData, &scraperErrors)

	return metrics, scraperErrors.Combine()
}

// scrapeScalarMetrics retrieves all SNMP data from scalar OIDs and turns the returned scalar data
// into metrics with optional enum attributes
func (s *snmpScraper) scrapeScalarMetrics(
	now pcommon.Timestamp,
	otelMetricData *snmpOTELMetricData,
	scraperErrors *scrapererror.ScrapeErrors,
) {
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
		return
	}

	// Retrieve all SNMP data from scalar metric OIDs
	scalarData := s.client.GetScalarData(scalarMetricOIDs, scraperErrors)
	// For each piece of SNMP data, attempt to create the necessary OTEL structures (resources/metrics/datapoints)
	for _, data := range scalarData {
		if err := s.scalarDataToMetric(data, now, otelMetricData, scalarMetricNamesByOID); err != nil {
			scraperErrors.AddPartial(1, fmt.Errorf(errMsgScalarOIDProcessing, data.oid, err))
		}
	}
}

// scalarDataToMetric will take one piece of SNMP scalar data and turn it into a datapoint for
// either a new or existing metric with attributes based on the related configs
func (s *snmpScraper) scalarDataToMetric(
	data SNMPData,
	now pcommon.Timestamp,
	otelMetricData *snmpOTELMetricData,
	scalarMetricNamesByOID map[string]string,
) error {
	// Return an error if this SNMP scalar data is not of a useable type
	if data.valueType == notSupportedVal || data.valueType == stringVal {
		return fmt.Errorf(errMsgBadValueType, data.oid)
	}

	// Retrieve the metric config for this SNMP data
	metricName := scalarMetricNamesByOID[data.oid]
	metricCfg := s.cfg.Metrics[metricName]

	// Get all enum attributes names and values for this SNMP scalar data based on the metric config
	var metricAttributes []Attribute
	for _, scalarOID := range metricCfg.ScalarOIDs {
		if scalarOID.OID == data.oid {
			metricAttributes = scalarOID.Attributes
		}
	}

	// Create new general resource if needed and get related MetricSlice
	var metricSlice pmetric.MetricSlice
	resourcesByKey := otelMetricData.resourcesByKey
	if resourcesByKey[generalResourceKey] == nil {
		generalResource := otelMetricData.resourceMetricsSlice.AppendEmpty()
		resourcesByKey[generalResourceKey] = &generalResource
		scopeMetrics := generalResource.ScopeMetrics().AppendEmpty()
		scopeMetrics.Scope().SetName("otelcol/snmpreceiver")
		scopeMetrics.Scope().SetVersion(s.settings.BuildInfo.Version)
		metricSlice = scopeMetrics.Metrics()
	} else {
		metricSlice = resourcesByKey[generalResourceKey].ScopeMetrics().At(0).Metrics()
	}

	// Get/create a metric and create a new datapoint for this metric using this SNMP scalar data
	metricsByName := otelMetricData.metricsByResource.getMetrics(generalResourceKey)
	metric := metricsByName[metricName]
	if metric == nil {
		metric = createMetric(&metricSlice, metricsByName, metricName, metricCfg)
	}
	dp, err := addMetricDataPoint(metric, metricCfg, now, data)
	if err != nil {
		return err
	}

	// Set enum attributes for this metric's datapoint based on the previously gathered attributes.
	// Keys will be determined from the related attribute config and enum values will come straight from
	// the metric config's attributes.
	for _, attribute := range metricAttributes {
		attributeCfg := s.cfg.Attributes[attribute.Name]
		attributeKey := attribute.Name
		if attributeCfg.Value != "" {
			attributeKey = attributeCfg.Value
		}
		dp.Attributes().PutStr(attributeKey, attribute.Value)
	}

	return nil
}

// scrapeIndexedMetrics retrieves all SNMP data from column OIDs and turns the returned indexed data
// into metrics with optional attribute and/or resource attributes
func (s *snmpScraper) scrapeIndexedMetrics(
	now pcommon.Timestamp,
	otelMetricData *snmpOTELMetricData,
	scraperErrors *scrapererror.ScrapeErrors,
) {
	// Retrieve column OID SNMP indexed data for attributes
	indexedAttributeValues := map[indexedAttributeKey]string{}
	s.scrapeIndexedAttributes(indexedAttributeValues, scraperErrors)

	// Retrieve column OID SNMP indexed data for resource attributes
	indexedResourceAttributeValues := map[indexedAttributeKey]string{}
	s.scrapeIndexedResourceAttributes(indexedResourceAttributeValues, scraperErrors)

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
		return
	}

	// Retrieve all SNMP indexed data from column metric OIDs
	indexedData := s.client.GetIndexedData(indexedMetricOIDs, scraperErrors)
	// For each piece of SNMP data, attempt to create the necessary OTEL structures (resources/metrics/datapoints)
	for _, data := range indexedData {
		if err := s.indexedDataToMetric(data, now, otelMetricData, indexedMetricNamesByOID, indexedAttributeValues, indexedResourceAttributeValues); err != nil {
			scraperErrors.AddPartial(1, fmt.Errorf(errMsgIndexedMetricOIDProcessing, data.oid, data.parentOID, err))
		}
	}
}

// indexedDataToMetric will take one piece of column OID SNMP indexed metric data and turn it
// into a datapoint for either a new or existing metric with attributes that belongs to either
// a new or existing resource
func (s *snmpScraper) indexedDataToMetric(
	data SNMPData,
	now pcommon.Timestamp,
	otelMetricData *snmpOTELMetricData,
	indexedMetricNamesByOID map[string]string,
	indexedAttributeValues map[indexedAttributeKey]string,
	indexedResourceAttributeValues map[indexedAttributeKey]string,
) error {
	// Return an error if this SNMP indexed data is not of a useable type
	if data.valueType == notSupportedVal || data.valueType == stringVal {
		return fmt.Errorf(errMsgIndexedBadValueType, data.oid, data.parentOID)
	}

	// Retrieve the metric config for this SNMP indexed data
	metricName := indexedMetricNamesByOID[data.parentOID]
	metricCfg := s.cfg.Metrics[metricName]

	// Get all related attribute and resource attribute info for this SNMP indexed data based on the metric config
	var metricCfgResourceAttributes []string
	var metricCfgAttributes []Attribute
	for _, columnOID := range metricCfg.ColumnOIDs {
		if columnOID.OID == data.parentOID {
			metricCfgAttributes = columnOID.Attributes
			metricCfgResourceAttributes = columnOID.ResourceAttributes
		}
	}

	indexString := strings.TrimPrefix(data.oid, data.parentOID)

	// Get data point attributes
	dataPointAttributes, err := getDataPointAttributes(indexedAttributeValues, metricCfgAttributes, s.cfg, indexString)
	if err != nil {
		return fmt.Errorf(errMsgOIDAttributeEmptyValue, metricName, err)
	}

	// Get resource attributes
	resourceAttributes, err := getResourceAttributes(indexedResourceAttributeValues, metricCfgResourceAttributes, s.cfg, indexString)
	if err != nil {
		return fmt.Errorf(errMsgOIDResourceAttributeEmptyValue, metricName, err)
	}

	// Create a resource key using all of the relevant resource attribute names along
	// with the row index of the SNMP data
	resourceKey := getResourceKey(metricCfgResourceAttributes, indexString)

	// Get/create the resource for this SNMP data
	resourcesByKey := otelMetricData.resourcesByKey
	resource := resourcesByKey[resourceKey]
	if resource == nil {
		resource = s.createResource(otelMetricData.resourceMetricsSlice, resourceAttributes, resourcesByKey, resourceKey)
	}
	metricSlice := resource.ScopeMetrics().At(0).Metrics()

	// Get/create the metric and datapoint for this SNMP data
	metricsByName := otelMetricData.metricsByResource.getMetrics(resourceKey)
	metric := metricsByName[metricName]
	if metric == nil {
		metric = createMetric(&metricSlice, metricsByName, metricName, metricCfg)
	}
	dp, err := addMetricDataPoint(metric, metricCfg, now, data)
	if err != nil {
		return err
	}

	// Add attributes to the datapoint if they exist
	for key, val := range dataPointAttributes {
		dp.Attributes().PutStr(key, val)
	}

	return nil
}

// getResourceKey returns a unique key based on all of the relevant resource attributes names
// as well as the current row index for a metric data point (a row index corresponds to values
// for the attributes that all belong to a single resource)
func getResourceKey(
	metricCfgResourceAttributes []string,
	indexString string,
) string {
	sort.Strings(metricCfgResourceAttributes)
	resourceKey := generalResourceKey
	if len(metricCfgResourceAttributes) > 0 {
		resourceKey = strings.Join(metricCfgResourceAttributes, " ") + indexString
	}

	return resourceKey
}

// getDataPointAttributes gets attributes for this metric's datapoint based on the previously
// gathered attributes.
// Keys will be determined from the related attribute config and values will come a few
// different places.
// Enum attribute value - comes from the metric config's attribute data
// Indexed prefix attribute value - comes from the current SNMP data's index and the attribute
// config's prefix value
// Indexed OID attribute value - comes from the previously collected indexed attribute data
// using the current index and attribute config to access the correct value
func getDataPointAttributes(
	indexedAttributeValues map[indexedAttributeKey]string,
	metricCfgAttributes []Attribute,
	cfg *Config,
	indexString string,
) (map[string]string, error) {
	datapointAttributes := map[string]string{}

	for _, attribute := range metricCfgAttributes {
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
			if attributeValue == "" {
				return nil, errors.New(errMsgAttributeEmptyValue)
			}
		}
		datapointAttributes[attributeKey] = attributeValue
	}

	return datapointAttributes, nil
}

// getResourceAttributes creates a map of key/values for all related resource attributes. Keys
// will come directly from the metric config's resource attribute values. Values will come
// from the related attribute config's prefix value plus the index OR the previously collected
// resource attribute indexed data.
func getResourceAttributes(
	indexedResourceAttributeValues map[indexedAttributeKey]string,
	metricCfgResourceAttributes []string,
	cfg *Config,
	indexString string,
) (map[string]string, error) {
	resourceAttributes := map[string]string{}

	for _, attributeName := range metricCfgResourceAttributes {
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
			if attributeValue == "" {
				return nil, errors.New(errMsgResourceAttributeEmptyValue)
			}
		}
		resourceAttributes[attributeName] = attributeValue
	}

	return resourceAttributes, nil
}

// createMetric creates a new metric based on the passed in data, adds it to the passed in MetricSlice,
// and adds it to the passed in metric map
func createMetric(
	metricSlice *pmetric.MetricSlice,
	metricsByName map[string]*pmetric.Metric,
	metricName string,
	metricCfg *MetricConfig,
) *pmetric.Metric {
	newMetric := metricSlice.AppendEmpty()
	metricsByName[metricName] = &newMetric
	newMetric.SetName(metricName)
	newMetric.SetDescription(metricCfg.Description)
	newMetric.SetUnit(metricCfg.Unit)

	if metricCfg.Sum != nil {
		newMetric.SetEmptySum()
		newMetric.Sum().SetIsMonotonic(metricCfg.Sum.Monotonic)

		switch metricCfg.Sum.Aggregation {
		case "cumulative":
			newMetric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		case "delta":
			newMetric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		}
	} else {
		newMetric.SetEmptyGauge()
	}

	return &newMetric
}

// createResource creates a new ResourceMetrics, adds it to the passed in ResourceMetricSlice,
// and adds it to the passed in resourceByKey
func (s *snmpScraper) createResource(
	resourceMetricsSlice *pmetric.ResourceMetricsSlice,
	resourceAttributes map[string]string,
	resourcesByKey map[string]*pmetric.ResourceMetrics,
	resourceKey string,
) *pmetric.ResourceMetrics {
	resourceMetrics := resourceMetricsSlice.AppendEmpty()
	for key, value := range resourceAttributes {
		resourceMetrics.Resource().Attributes().PutStr(key, value)
	}
	scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()
	scopeMetrics.Scope().SetName("otelcol/snmpreceiver")
	scopeMetrics.Scope().SetVersion(s.settings.BuildInfo.Version)
	resourcesByKey[resourceKey] = &resourceMetrics

	return &resourceMetrics
}

// addMetricDataPoint adds a new metric data point to the passed in Metric given the data
func addMetricDataPoint(
	metric *pmetric.Metric,
	metricCfg *MetricConfig,
	now pcommon.Timestamp,
	data SNMPData,
) (*pmetric.NumberDataPoint, error) {
	if metric == nil {
		return nil, fmt.Errorf("cannot retrieve datapoints from metric '%s' as it does not currently exist", metric.Name())
	}

	var dps pmetric.NumberDataPointSlice
	if metricCfg.Gauge != nil {
		dps = metric.Gauge().DataPoints()
	} else {
		dps = metric.Sum().DataPoints()
	}

	// Creates a data point based on the SNMP data
	dp := dps.AppendEmpty()
	dp.SetTimestamp(now)
	// Not explicitly checking these casts as this should be made safe in the client
	switch data.valueType {
	case floatVal:
		dp.SetDoubleValue(data.value.(float64))
	default:
		dp.SetIntValue(data.value.(int64))
	}

	return &dp, nil
}

// scrapeIndexedAttributes retrieves all SNMP data from attribute config column OIDs and
// stores the returned indexed data for later use by metrics
func (s *snmpScraper) scrapeIndexedAttributes(
	indexedAttributeValues map[indexedAttributeKey]string,
	scraperErrors *scrapererror.ScrapeErrors) {
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
		return
	}

	// Retrieve all SNMP indexed data from column attribute OIDs
	indexedData := s.client.GetIndexedData(indexedAttributeOIDs, scraperErrors)
	// For each piece of SNMP data, store the necessary info to help create metric attributes later if needed
	for _, data := range indexedData {
		if err := indexedDataToAttribute(data, indexedAttributeValues); err != nil {
			scraperErrors.AddPartial(1, fmt.Errorf(errMsgIndexedAttributeOIDProcessing, data.oid, data.parentOID, err))
		}
	}
}

// scrapeIndexedResourceAttributes retrieves all SNMP data from resource attribute config column OIDs and
// stores the returned indexed data for later use by metrics
func (s *snmpScraper) scrapeIndexedResourceAttributes(
	indexedResourceAttributeValues map[indexedAttributeKey]string,
	scraperErrors *scrapererror.ScrapeErrors) {
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
		return
	}

	// Retrieve all SNMP indexed data from column resource attribute OIDs
	indexedData := s.client.GetIndexedData(indexedResourceAttributeOIDs, scraperErrors)
	// For each piece of SNMP data, store the necessary info to help create resources later if needed
	for _, data := range indexedData {
		if err := indexedDataToAttribute(data, indexedResourceAttributeValues); err != nil {
			scraperErrors.AddPartial(1, fmt.Errorf(errMsgIndexedResourceAttributeOIDProcessing, data.oid, data.parentOID, err))
		}
	}
}

// indexedDataToAttribute provides a function which will take one piece of column OID SNMP indexed data
// (for either an attribute or resource attribute) and stores it in a map for later use (keyed by both
// {resource} attribute config column OID and OID index)
func indexedDataToAttribute(
	data SNMPData,
	indexedAttributeValues map[indexedAttributeKey]string,
) error {
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
