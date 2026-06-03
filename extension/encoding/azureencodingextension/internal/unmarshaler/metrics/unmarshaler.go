// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler/metrics"

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
	"time"

	gojson "github.com/goccy/go-json"
	jsoniter "github.com/json-iterator/go"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler"
)

var timeGrains = map[string]int64{
	"PT1M":  60,
	"PT5M":  300,
	"PT15M": 900,
	"PT30M": 1800,
	"PT1H":  3600,
	"PT6H":  21600,
	"PT12H": 43200,
	"P1D":   86400,
}

// azureMetricRecord represents a single Azure Metric following
// the common schema does not exist (yet):
type azureMetricRecord struct {
	Time       string  `json:"time"`
	ResourceID string  `json:"resourceId"`
	MetricName string  `json:"metricName"`
	TimeGrain  string  `json:"timeGrain"`
	Total      float64 `json:"total"`
	Count      float64 `json:"count"`
	Minimum    float64 `json:"minimum"`
	Maximum    float64 `json:"maximum"`
	Average    float64 `json:"average"`
	// Fields below are available only via DCR, not via Diagnostic Settings
	Unit       string         `json:"unit"`
	Dimensions map[string]any `json:"dimension"`
}

type ResourceMetricsUnmarshaler struct {
	buildInfo    component.BuildInfo
	logger       *zap.Logger
	timeFormat   []string
	aggregations []MetricAggregation
}

// UnmarshalMetrics takes a byte array containing a JSON-encoded
// payload with Azure metric records and transforms it into
// an OpenTelemetry pmetric.Metrics object. The data in the Azure
// metric record appears as fields and attributes in the
// OpenTelemetry representation
func (r ResourceMetricsUnmarshaler) UnmarshalMetrics(buf []byte) (pmetric.Metrics, error) {
	allResourceScopeMetrics := map[string]pmetric.ScopeMetrics{}

	batchFormat, err := unmarshaler.DetectWrapperFormat(buf)
	if err != nil {
		return pmetric.NewMetrics(), err
	}

	switch batchFormat {
	// ND JSON is a specific case...
	// We will use bufio.Scanner trick to read it line by line
	// as unmarshal each line as a Metric Record
	case unmarshaler.FormatNDJSON:
		scanner := bufio.NewScanner(bytes.NewReader(buf))
		for scanner.Scan() {
			r.unmarshalRecord(allResourceScopeMetrics, scanner.Bytes())
		}
	// Both formats are valid JSON and can be parsed directly
	// `gojson.Path.Extract` is a bit faster and use ~25% less bytes per operation
	// comparing to unmarshaling to intermediate structure (e.g. using `var recordsHolder []json.RawMessage`)
	case unmarshaler.FormatObjectRecords, unmarshaler.FormatJSONArray:
		jsonPath := unmarshaler.JSONPathEventHubRecords
		if batchFormat == unmarshaler.FormatJSONArray {
			jsonPath = unmarshaler.JSONPathBlobStorageRecords
		}

		// This will allow us to parse Azure Metric Records in both formats:
		// 1) As exported to Azure Event Hub, e.g. `{"records": [ {...}, {...} ]}`
		// 2) As exported to Azure Blob Storage, e.g. `[ {...}, {...} ]`
		rootPath, err := gojson.CreatePath(jsonPath)
		if err != nil {
			// This should never happen, but still...
			return pmetric.NewMetrics(), fmt.Errorf("failed to create JSON Path %q: %w", jsonPath, err)
		}

		records, err := rootPath.Extract(buf)
		if err != nil {
			// This should never happen, but still...
			return pmetric.NewMetrics(), fmt.Errorf("failed to extract Azure Metrics Records: %w", err)
		}

		for _, record := range records {
			r.unmarshalRecord(allResourceScopeMetrics, record)
		}
	// This happens on empty input
	case unmarshaler.FormatUnknown:
		return pmetric.NewMetrics(), nil
	default:
		return pmetric.NewMetrics(), fmt.Errorf("unrecognized batch format: %q", batchFormat)
	}

	m := pmetric.NewMetrics()
	for resourceID, scopeMetrics := range allResourceScopeMetrics {
		// Do not add empty ResourceMetrics
		// This can happen if there are no metrics for a given ResourceID
		// or timeGrain/timestamp is invalid
		if scopeMetrics.Metrics().Len() == 0 {
			continue
		}

		rm := m.ResourceMetrics().AppendEmpty()
		rm.Resource().Attributes().PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderAzure.Value.AsString())
		rm.Resource().Attributes().PutStr(string(conventions.CloudResourceIDKey), resourceID)
		scopeMetrics.MoveTo(rm.ScopeMetrics().AppendEmpty())
	}

	return m, nil
}

func (r ResourceMetricsUnmarshaler) unmarshalRecord(allResourceScopeMetrics map[string]pmetric.ScopeMetrics, record []byte) {
	var azureMetric azureMetricRecord

	if err := jsoniter.ConfigFastest.Unmarshal(record, &azureMetric); err != nil {
		r.logger.Error("JSON unmarshal failed for Azure Metrics Record", zap.Error(err))
		return
	}

	if azureMetric.ResourceID == "" {
		r.logger.Warn(
			"No ResourceID set on Metrics record",
			zap.String("metricName", azureMetric.MetricName),
		)
	}

	// Grouping set of metrics by Azure ResourceID's
	scopeMetrics, found := allResourceScopeMetrics[azureMetric.ResourceID]
	if !found {
		scopeMetrics = pmetric.NewScopeMetrics()
		scopeMetrics.Scope().SetName(metadata.ScopeName)
		scopeMetrics.Scope().SetVersion(r.buildInfo.Version)
		scopeMetrics.Scope().Attributes().PutStr(constants.FormatIdentificationTag, constants.ProviderFromResourceID(azureMetric.ResourceID))
		allResourceScopeMetrics[azureMetric.ResourceID] = scopeMetrics
	}

	timestamp, err := unmarshaler.AsTimestamp(azureMetric.Time, r.timeFormat...)
	if err != nil {
		r.logger.Error(
			"Unable to parse timestamp from Azure Metric",
			zap.String("time", azureMetric.Time),
			zap.Error(err),
		)
		return
	}
	if _, found := timeGrains[azureMetric.TimeGrain]; !found {
		r.logger.Error(
			"Unhandled Time Grain",
			zap.String("timeGrain", azureMetric.TimeGrain),
			zap.String("metricName", azureMetric.MetricName),
		)
		return
	}
	startTimestamp := pcommon.NewTimestampFromTime(timestamp.AsTime().Add(-time.Duration(timeGrains[azureMetric.TimeGrain]) * time.Second))

	metrics := scopeMetrics.Metrics()
	for _, agg := range r.aggregations {
		m := metrics.AppendEmpty()
		m.SetName(strings.ToLower(fmt.Sprintf("%s_%s", strings.ReplaceAll(azureMetric.MetricName, " ", "_"), agg)))
		dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
		dp.SetStartTimestamp(startTimestamp)
		dp.SetTimestamp(timestamp)
		switch agg {
		case AggregationTotal:
			dp.SetDoubleValue(azureMetric.Total)
		case AggregationCount:
			dp.SetDoubleValue(azureMetric.Count)
		case AggregationMinimum:
			dp.SetDoubleValue(azureMetric.Minimum)
		case AggregationMaximum:
			dp.SetDoubleValue(azureMetric.Maximum)
		case AggregationAverage:
			dp.SetDoubleValue(azureMetric.Average)
		}
		// Only for records exported via DCRs
		if azureMetric.Unit != "" {
			m.SetUnit(azureMetric.Unit)
		}
		if len(azureMetric.Dimensions) > 0 {
			if err := dp.Attributes().FromRaw(azureMetric.Dimensions); err != nil {
				r.logger.Warn(
					"Failed to set attributes from dimensions",
					zap.Any("dimensions", azureMetric.Dimensions),
					zap.Error(err),
				)
			}
		}
	}
}

func NewAzureResourceMetricsUnmarshaler(buildInfo component.BuildInfo, logger *zap.Logger, cfg MetricsConfig) ResourceMetricsUnmarshaler {
	agg := cfg.Aggregations
	if len(agg) == 0 {
		agg = supportedAggregations
	}

	return ResourceMetricsUnmarshaler{
		buildInfo:    buildInfo,
		logger:       logger,
		timeFormat:   cfg.TimeFormats,
		aggregations: agg,
	}
}
