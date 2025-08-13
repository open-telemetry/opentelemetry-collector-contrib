// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	eventhub "github.com/Azure/azure-event-hubs-go/v3"
	jsoniter "github.com/json-iterator/go"
	"github.com/relvacode/iso8601"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/otel/semconv/v1.27.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver/internal/metadata"
)

const azureResourceID = "azure.resource.id"

type azureResourceMetricsUnmarshaler struct {
	buildInfo  component.BuildInfo
	logger     *zap.Logger
	TimeFormat []string
}

// azureMetricRecords represents an array of Azure metric records
// as exported via an Azure Event Hub
type azureMetricRecords struct {
	Records []azureMetricRecord `json:"records"`
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
}

func newAzureResourceMetricsUnmarshaler(buildInfo component.BuildInfo, logger *zap.Logger, timeFormat []string) eventMetricsUnmarshaler {
	return azureResourceMetricsUnmarshaler{
		buildInfo:  buildInfo,
		logger:     logger,
		TimeFormat: timeFormat,
	}
}

// UnmarshalMetrics takes a byte array containing a JSON-encoded
// payload with Azure metric records and transforms it into
// an OpenTelemetry pmetric.Metrics object. The data in the Azure
// metric record appears as fields and attributes in the
// OpenTelemetry representation;
func (r azureResourceMetricsUnmarshaler) UnmarshalMetrics(event *eventhub.Event) (pmetric.Metrics, error) {
	md := pmetric.NewMetrics()

	var azureMetrics azureMetricRecords
	decoder := jsoniter.NewDecoder(bytes.NewReader(event.Data))
	err := decoder.Decode(&azureMetrics)
	if err != nil {
		return md, err
	}

	resourceMetrics := md.ResourceMetrics().AppendEmpty()
	resource := resourceMetrics.Resource()
	resource.Attributes().PutStr(string(conventions.TelemetrySDKNameKey), metadata.ScopeName)
	resource.Attributes().PutStr(string(conventions.TelemetrySDKLanguageKey), conventions.TelemetrySDKLanguageGo.Value.AsString())
	resource.Attributes().PutStr(string(conventions.TelemetrySDKVersionKey), r.buildInfo.Version)
	resource.Attributes().PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderAzure.Value.AsString())

	scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()

	metrics := scopeMetrics.Metrics()
	metrics.EnsureCapacity(len(azureMetrics.Records) * 5)

	resourceID := ""
	for _, azureMetric := range azureMetrics.Records {
		if resourceID == "" && azureMetric.ResourceID != "" {
			resourceID = azureMetric.ResourceID
		}

		nanos, err := asTimestamp(azureMetric.Time, r.TimeFormat)
		if err != nil {
			r.logger.Warn("Invalid Timestamp", zap.String("time", azureMetric.Time))
			continue
		}

		var startTimestamp pcommon.Timestamp
		if azureMetric.TimeGrain != "PT1M" {
			r.logger.Warn("Unhandled Time Grain", zap.String("timegrain", azureMetric.TimeGrain))
			continue
		}
		startTimestamp = pcommon.NewTimestampFromTime(nanos.AsTime().Add(-time.Minute))

		metricTotal := metrics.AppendEmpty()
		metricTotal.SetName(strings.ToLower(fmt.Sprintf("%s_%s", strings.ReplaceAll(azureMetric.MetricName, " ", "_"), "Total")))
		dpTotal := metricTotal.SetEmptyGauge().DataPoints().AppendEmpty()
		dpTotal.SetStartTimestamp(startTimestamp)
		dpTotal.SetTimestamp(nanos)
		dpTotal.SetDoubleValue(azureMetric.Total)

		metricCount := metrics.AppendEmpty()
		metricCount.SetName(strings.ToLower(fmt.Sprintf("%s_%s", strings.ReplaceAll(azureMetric.MetricName, " ", "_"), "Count")))
		dpCount := metricCount.SetEmptyGauge().DataPoints().AppendEmpty()
		dpCount.SetStartTimestamp(startTimestamp)
		dpCount.SetTimestamp(nanos)
		dpCount.SetDoubleValue(azureMetric.Count)

		metricMin := metrics.AppendEmpty()
		metricMin.SetName(strings.ToLower(fmt.Sprintf("%s_%s", strings.ReplaceAll(azureMetric.MetricName, " ", "_"), "Minimum")))
		dpMin := metricMin.SetEmptyGauge().DataPoints().AppendEmpty()
		dpMin.SetStartTimestamp(startTimestamp)
		dpMin.SetTimestamp(nanos)
		dpMin.SetDoubleValue(azureMetric.Minimum)

		metricMax := metrics.AppendEmpty()
		metricMax.SetName(strings.ToLower(fmt.Sprintf("%s_%s", strings.ReplaceAll(azureMetric.MetricName, " ", "_"), "Maximum")))
		dpMax := metricMax.SetEmptyGauge().DataPoints().AppendEmpty()
		dpMax.SetStartTimestamp(startTimestamp)
		dpMax.SetTimestamp(nanos)
		dpMax.SetDoubleValue(azureMetric.Maximum)

		metricAverage := metrics.AppendEmpty()
		metricAverage.SetName(strings.ToLower(fmt.Sprintf("%s_%s", strings.ReplaceAll(azureMetric.MetricName, " ", "_"), "Average")))
		dpAverage := metricAverage.SetEmptyGauge().DataPoints().AppendEmpty()
		dpAverage.SetStartTimestamp(startTimestamp)
		dpAverage.SetTimestamp(nanos)
		dpAverage.SetDoubleValue(azureMetric.Average)
	}

	if resourceID != "" {
		resourceMetrics.Resource().Attributes().PutStr(azureResourceID, resourceID)
	} else {
		r.logger.Warn("No ResourceID Set on Metrics!")
	}

	return md, nil
}

// asTimestamp will parse an ISO8601 string into an OpenTelemetry
// nanosecond timestamp. If the string cannot be parsed, it will
// return zero and the error.
func asTimestamp(s string, formats []string) (pcommon.Timestamp, error) {
	var err error
	var t time.Time
	// Try parsing with provided formats first
	for _, format := range formats {
		if t, err = time.Parse(format, s); err == nil {
			return pcommon.Timestamp(t.UnixNano()), nil
		}
	}

	// Fallback to ISO 8601 parsing if no format matches
	if t, err = iso8601.ParseString(s); err == nil {
		return pcommon.Timestamp(t.UnixNano()), nil
	}
	return 0, err
}
