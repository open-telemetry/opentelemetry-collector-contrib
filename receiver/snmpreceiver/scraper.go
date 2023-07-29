// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package snmpreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver"

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/zap"
)

var (
	// Error messages
	errMsgBadValueType                   = `returned metric SNMP data type for OID '%s' is not supported`
	errMsgIndexedAttributesBadValueType  = `returned attribute SNMP data type for OID '%s' from column OID '%s' is not supported`
	errMsgOIDAttributeEmptyValue         = `not creating indexed metric '%s' datapoint: %w`
	errMsgAttributeEmptyValue            = `metric OID attribute value is blank`
	errMsgResourceAttributeEmptyValue    = `related resource attribute value is blank`
	errMsgOIDResourceAttributeEmptyValue = `not creating indexed metric '%s' or resource: %w`
	errMsgScalarOIDProcessing            = `problem processing scalar metric data for OID '%s': %w`
	errMsgIndexedMetricOIDProcessing     = `problem processing indexed metric data for OID '%s' from column OID '%s': %w`
	errMsgIndexedAttributeOIDProcessing  = `problem processing indexed attribute data for OID '%s' from column OID '%s': %w`
)

// snmpScraper handles scraping of SNMP metrics
type snmpScraper struct {
	client    client
	logger    *zap.Logger
	cfg       *Config
	settings  receiver.CreateSettings
	startTime pcommon.Timestamp
}

type indexedAttributeValues map[string]string

// newScraper creates an initialized snmpScraper
func newScraper(logger *zap.Logger, cfg *Config, settings receiver.CreateSettings) *snmpScraper {
	return &snmpScraper{
		logger:   logger,
		cfg:      cfg,
		settings: settings,
	}
}

// start gets the client ready
func (s *snmpScraper) start(_ context.Context, _ component.Host) (err error) {
	s.client, err = newClient(s.cfg, s.logger)
	s.startTime = pcommon.NewTimestampFromTime(time.Now())
	return err
}

// scrape collects and creates OTEL metrics from a SNMP environment
func (s *snmpScraper) scrape(_ context.Context) (pmetric.Metrics, error) {
	if err := s.client.Connect(); err != nil {
		return pmetric.NewMetrics(), fmt.Errorf("problem connecting to SNMP host: %w", err)
	}
	defer s.client.Close()

	// Create the metrics helper which will help manage a lot of the otel metric and resource functionality
	metricHelper := newOTELMetricHelper(s.settings, s.startTime)

	configHelper := newConfigHelper(s.cfg)

	var scraperErrors scrapererror.ScrapeErrors
	// Try to scrape scalar OID based metrics
	s.scrapeScalarMetrics(metricHelper, configHelper, &scraperErrors)

	// Try to scrape column OID based metrics
	s.scrapeIndexedMetrics(metricHelper, configHelper, &scraperErrors)

	return metricHelper.metrics, scraperErrors.Combine()
}

// scrapeScalarMetrics retrieves all SNMP data from scalar OIDs and turns the returned scalar data
// into metrics with optional enum attributes
func (s *snmpScraper) scrapeScalarMetrics(
	metricHelper *otelMetricHelper,
	configHelper *configHelper,
	scraperErrors *scrapererror.ScrapeErrors,
) {
	metricScalarOIDs := configHelper.getMetricScalarOIDs()

	// If no scalar metric configs, nothing else to do
	if len(metricScalarOIDs) == 0 {
		return
	}

	// Retrieve all SNMP data from scalar metric OIDs
	scalarData := s.client.GetScalarData(metricScalarOIDs, scraperErrors)

	// If no scalar data, nothing else to do
	if len(scalarData) == 0 {
		return
	}

	// Create general resource if we're about to make scalar metrics
	resource := metricHelper.getResource(generalResourceKey)
	if resource == nil {
		metricHelper.createResource(generalResourceKey, map[string]string{})
	}

	// For each piece of SNMP data, attempt to create the necessary OTEL structures (resources/metrics/datapoints)
	for _, data := range scalarData {
		if err := s.scalarDataToMetric(data, metricHelper, configHelper); err != nil {
			scraperErrors.AddPartial(1, fmt.Errorf(errMsgScalarOIDProcessing, data.oid, err))
		}
	}
}

// scrapeIndexedMetrics retrieves all SNMP data from column OIDs and turns the returned indexed data
// into metrics with optional attribute and/or resource attributes
func (s *snmpScraper) scrapeIndexedMetrics(
	metricHelper *otelMetricHelper,
	configHelper *configHelper,
	scraperErrors *scrapererror.ScrapeErrors,
) {
	metricColumnOIDs := configHelper.getMetricColumnOIDs()

	// If no column metric configs, nothing else to do
	if len(metricColumnOIDs) == 0 {
		return
	}

	// Retrieve column OID SNMP indexed data for attributes
	columnOIDIndexedAttributeValues := s.scrapeIndexedAttributes(configHelper.getAttributeColumnOIDs(), scraperErrors)

	// Retrieve column OID SNMP indexed data for resource attributes
	columnOIDIndexedResourceAttributeValues := s.scrapeIndexedAttributes(configHelper.getResourceAttributeColumnOIDs(), scraperErrors)

	// Retrieve all SNMP indexed data from column metric OIDs
	indexedData := s.client.GetIndexedData(metricColumnOIDs, scraperErrors)
	// For each piece of SNMP data, attempt to create the necessary OTEL structures (resources/metrics/datapoints)
	for _, data := range indexedData {
		if err := s.indexedDataToMetric(data, metricHelper, configHelper, columnOIDIndexedAttributeValues, columnOIDIndexedResourceAttributeValues); err != nil {
			scraperErrors.AddPartial(1, fmt.Errorf(errMsgIndexedMetricOIDProcessing, data.oid, data.columnOID, err))
		}
	}
}

// scalarDataToMetric will take one piece of SNMP scalar data and turn it into a datapoint for
// either a new or existing metric with attributes based on the related configs
func (s *snmpScraper) scalarDataToMetric(
	data SNMPData,
	metricHelper *otelMetricHelper,
	configHelper *configHelper,
) error {
	// Get the related metric name for this SNMP indexed data
	metricName := configHelper.getMetricName(data.oid)

	// Keys will be determined from the related attribute config and enum values will come straight from
	// the metric config's attribute values.
	dataPointAttributes := getScalarDataPointAttributes(configHelper, data.oid)

	return addMetricDataPointToResource(data, metricHelper, configHelper, metricName, generalResourceKey, dataPointAttributes)
}

// indexedDataToMetric will take one piece of column OID SNMP indexed metric data and turn it
// into a datapoint for either a new or existing metric with attributes that belongs to either
// a new or existing resource
func (s *snmpScraper) indexedDataToMetric(
	data SNMPData,
	metricHelper *otelMetricHelper,
	configHelper *configHelper,
	columnOIDIndexedAttributeValues map[string]indexedAttributeValues,
	columnOIDIndexedResourceAttributeValues map[string]indexedAttributeValues,
) error {
	// Get the related metric name for this SNMP indexed data
	metricName := configHelper.getMetricName(data.columnOID)

	indexString := strings.TrimPrefix(data.oid, data.columnOID)

	// Get data point attributes
	dataPointAttributes, err := getIndexedDataPointAttributes(configHelper, data.columnOID, indexString, columnOIDIndexedAttributeValues)
	if err != nil {
		return fmt.Errorf(errMsgOIDAttributeEmptyValue, metricName, err)
	}

	// Get resource attributes
	resourceAttributes, err := getResourceAttributes(configHelper, data.columnOID, indexString, columnOIDIndexedResourceAttributeValues)
	if err != nil {
		return fmt.Errorf(errMsgOIDResourceAttributeEmptyValue, metricName, err)
	}

	// Create a resource key using all of the relevant resource attribute names along
	// with the row index of the SNMP data
	resourceAttributeNames := configHelper.getResourceAttributeNames(data.columnOID)
	resourceKey := getResourceKey(resourceAttributeNames, indexString)

	// Create a new resource if needed
	resource := metricHelper.getResource(resourceKey)
	if resource == nil {
		metricHelper.createResource(resourceKey, resourceAttributes)
	}

	return addMetricDataPointToResource(data, metricHelper, configHelper, metricName, resourceKey, dataPointAttributes)
}

func addMetricDataPointToResource(
	data SNMPData,
	metricHelper *otelMetricHelper,
	configHelper *configHelper,
	metricName string,
	resourceKey string,
	dataPointAttributes map[string]string,
) error {
	// Return an error if this SNMP indexed data is not of a useable type
	if data.valueType == notSupportedVal || data.valueType == stringVal {
		return fmt.Errorf(errMsgBadValueType, data.oid)
	}

	// Get the related metric config
	metricCfg := configHelper.getMetricConfig(metricName)

	// Create a new metric if needed
	if metric := metricHelper.getMetric(resourceKey, metricName); metric == nil {
		if _, err := metricHelper.createMetric(resourceKey, metricName, metricCfg); err != nil {
			return err
		}
	}

	// Add data point to metric
	if _, err := metricHelper.addMetricDataPoint(resourceKey, metricName, metricCfg, data, dataPointAttributes); err != nil {
		return err
	}

	return nil
}

// getScalarDataPointAttributes returns the key value pairs of attributes for a given metric config scalar OID
func getScalarDataPointAttributes(configHelper *configHelper, oid string) map[string]string {
	dataPointAttributes := map[string]string{}
	for _, attribute := range configHelper.metricAttributesByOID[oid] {
		attributeKey := attribute.Name
		if value := configHelper.getAttributeConfigValue(attributeKey); value != "" {
			attributeKey = value
		}
		dataPointAttributes[attributeKey] = attribute.Value
	}

	return dataPointAttributes
}

// getIndexedDataPointAttributes gets attributes for this metric's datapoint based on the previously
// gathered attributes.
// Keys will be determined from the related attribute config and values will come a few
// different places.
// Enum attribute value - comes from the metric config's attribute data
// Indexed prefix attribute value - comes from the current SNMP data's index and the attribute
// config's prefix value
// Indexed OID attribute value - comes from the previously collected indexed attribute data
// using the current index and attribute config to access the correct value
func getIndexedDataPointAttributes(
	configHelper *configHelper,
	columnOID string,
	indexString string,
	columnOIDIndexedAttributeValues map[string]indexedAttributeValues,
) (map[string]string, error) {
	datapointAttributes := map[string]string{}

	for _, attribute := range configHelper.getMetricConfigAttributes(columnOID) {
		attributeName := attribute.Name

		attributeKey := attributeName
		// Use alternate attribute key if available
		if value := configHelper.getAttributeConfigValue(attributeKey); value != "" {
			attributeKey = value
		}

		var attributeValue string
		prefix := configHelper.getAttributeConfigIndexedValuePrefix(attributeName)
		oid := configHelper.getAttributeConfigOID(attributeName)
		switch {
		case prefix != "":
			attributeValue = prefix + indexString
		case oid != "":
			attributeValue = columnOIDIndexedAttributeValues[oid][indexString]
		default:
			attributeValue = attribute.Value
		}

		// If no good attribute value could be found
		if attributeValue == "" {
			return nil, errors.New(errMsgAttributeEmptyValue)
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
	configHelper *configHelper,
	columnOID string,
	indexString string,
	columnOIDIndexedResourceAttributeValues map[string]indexedAttributeValues,
) (map[string]string, error) {
	resourceAttributes := map[string]string{}

	for _, attributeName := range configHelper.getResourceAttributeNames(columnOID) {
		prefix := configHelper.getResourceAttributeConfigIndexedValuePrefix(attributeName)
		oid := configHelper.getResourceAttributeConfigOID(attributeName)
		switch {
		case prefix != "":
			resourceAttributes[attributeName] = prefix + indexString
		case oid != "":
			attributeValue := columnOIDIndexedResourceAttributeValues[oid][indexString]

			if attributeValue == "" {
				return nil, errors.New(errMsgResourceAttributeEmptyValue)
			}

			resourceAttributes[attributeName] = attributeValue
		default:
			return nil, errors.New(errMsgResourceAttributeEmptyValue)
		}
	}

	return resourceAttributes, nil
}

// scrapeIndexedAttributes retrieves all SNMP data from attribute (or resource attribute)
// config column OIDs and stores the returned indexed data for later use by metrics
func (s *snmpScraper) scrapeIndexedAttributes(
	columnOIDs []string,
	scraperErrors *scrapererror.ScrapeErrors,
) map[string]indexedAttributeValues {
	columnOIDIndexedAttributeValues := map[string]indexedAttributeValues{}

	// If no OID resource attribute configs, nothing else to do
	if len(columnOIDs) == 0 {
		return columnOIDIndexedAttributeValues
	}

	// Retrieve all SNMP indexed data from column resource attribute OIDs
	indexedData := s.client.GetIndexedData(columnOIDs, scraperErrors)

	// For each piece of SNMP data, store the necessary info to help create resources later if needed
	for _, data := range indexedData {
		if err := indexedDataToAttribute(data, columnOIDIndexedAttributeValues); err != nil {
			scraperErrors.AddPartial(1, fmt.Errorf(errMsgIndexedAttributeOIDProcessing, data.oid, data.columnOID, err))
		}
	}

	return columnOIDIndexedAttributeValues
}

// indexedDataToAttribute provides a function which will take one piece of column OID SNMP indexed data
// (for either an attribute or resource attribute) and stores it in a map for later use (keyed by both
// {resource} attribute config column OID and OID index)
func indexedDataToAttribute(
	data SNMPData,
	columnOIDIndexedAttributeValues map[string]indexedAttributeValues,
) error {
	// Get the string value of the SNMP data for the {resource} attribute value
	var stringValue string
	// Not explicitly checking these casts as this should be made safe in the client
	switch data.valueType {
	case notSupportedVal:
		return fmt.Errorf(errMsgIndexedAttributesBadValueType, data.oid, data.columnOID)
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
	indexString := strings.TrimPrefix(data.oid, data.columnOID)
	if columnOIDIndexedAttributeValues[data.columnOID] == nil {
		columnOIDIndexedAttributeValues[data.columnOID] = indexedAttributeValues{}
	}
	columnOIDIndexedAttributeValues[data.columnOID][indexString] = stringValue

	return nil
}
