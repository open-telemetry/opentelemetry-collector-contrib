// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	"bytes"
	"fmt"
	"strings"
	"time"

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

type azureResourceMetricsConfiger interface {
	GetLogger() *zap.Logger
	GetBuildVersion() string
	GetTimeFormat() []string
}

// azureMetricRecords represents an array of Azure metric records
// as exported via an Azure Event Hub
type azureMetricRecords struct {
	Records []azureGenericMetricRecord `json:"records"`
}

type azureMetricAppender interface {
	AppendMetric(azureResourceMetricsConfiger, *pmetric.Metrics) error
}

type azureGenericMetricRecord struct {
	Type   string
	Record azureMetricAppender
}

func (r *azureGenericMetricRecord) UnmarshalJSON(data []byte) error {
	var recordWithType struct {
		Type string `json:"Type"`
	}
	typeDecoder := jsoniter.NewDecoder(bytes.NewReader(data))
	err := typeDecoder.Decode(&recordWithType)

	if err != nil {
		return err
	}

	r.Type = recordWithType.Type

	switch r.Type {
	case "AppMetrics":
		r.Record = &azureAppMetricRecord{}
	default:
		r.Record = &azureResourceMetricRecord{}
	}

	recordDecoder := jsoniter.NewDecoder(bytes.NewReader(data))
	err = recordDecoder.Decode(r.Record)

	if err != nil {
		return err
	}

	return nil
}

// azureMetricRecord represents a single Azure Metric following
// the common schema does not exist (yet):
type azureResourceMetricRecord struct {
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

func (r *azureResourceMetricRecord) AppendMetric(c azureResourceMetricsConfiger, md *pmetric.Metrics) error {
	resourceMetrics := md.ResourceMetrics().AppendEmpty()

	resource := resourceMetrics.Resource()
	resource.Attributes().PutStr(string(conventions.TelemetrySDKNameKey), metadata.ScopeName)
	resource.Attributes().PutStr(string(conventions.TelemetrySDKLanguageKey), conventions.TelemetrySDKLanguageGo.Value.AsString())
	resource.Attributes().PutStr(string(conventions.TelemetrySDKVersionKey), c.GetBuildVersion())
	resource.Attributes().PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderAzure.Value.AsString())

	scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()

	metrics := scopeMetrics.Metrics()
	metrics.EnsureCapacity(5)

	if r.ResourceID != "" {
		resourceMetrics.Resource().Attributes().PutStr(azureResourceID, r.ResourceID)
	} else {
		c.GetLogger().Warn("No ResourceID Set on Metrics!")
	}

	nanos, err := asTimestamp(r.Time, c.GetTimeFormat())
	if err != nil {
		c.GetLogger().Warn("Invalid Timestamp", zap.String("time", r.Time))
		return err
	}

	var startTimestamp pcommon.Timestamp
	if r.TimeGrain != "PT1M" {
		c.GetLogger().Warn("Unhandled Time Grain", zap.String("timegrain", r.TimeGrain))
		return err
	}

	startTimestamp = pcommon.NewTimestampFromTime(nanos.AsTime().Add(-time.Minute))

	metricTotal := metrics.AppendEmpty()
	metricTotal.SetName(strings.ToLower(fmt.Sprintf("%s_%s", strings.ReplaceAll(r.MetricName, " ", "_"), "Total")))
	dpTotal := metricTotal.SetEmptyGauge().DataPoints().AppendEmpty()
	dpTotal.SetStartTimestamp(startTimestamp)
	dpTotal.SetTimestamp(nanos)
	dpTotal.SetDoubleValue(r.Total)

	metricCount := metrics.AppendEmpty()
	metricCount.SetName(strings.ToLower(fmt.Sprintf("%s_%s", strings.ReplaceAll(r.MetricName, " ", "_"), "Count")))
	dpCount := metricCount.SetEmptyGauge().DataPoints().AppendEmpty()
	dpCount.SetStartTimestamp(startTimestamp)
	dpCount.SetTimestamp(nanos)
	dpCount.SetDoubleValue(r.Count)

	metricMin := metrics.AppendEmpty()
	metricMin.SetName(strings.ToLower(fmt.Sprintf("%s_%s", strings.ReplaceAll(r.MetricName, " ", "_"), "Minimum")))
	dpMin := metricMin.SetEmptyGauge().DataPoints().AppendEmpty()
	dpMin.SetStartTimestamp(startTimestamp)
	dpMin.SetTimestamp(nanos)
	dpMin.SetDoubleValue(r.Minimum)

	metricMax := metrics.AppendEmpty()
	metricMax.SetName(strings.ToLower(fmt.Sprintf("%s_%s", strings.ReplaceAll(r.MetricName, " ", "_"), "Maximum")))
	dpMax := metricMax.SetEmptyGauge().DataPoints().AppendEmpty()
	dpMax.SetStartTimestamp(startTimestamp)
	dpMax.SetTimestamp(nanos)
	dpMax.SetDoubleValue(r.Maximum)

	metricAverage := metrics.AppendEmpty()
	metricAverage.SetName(strings.ToLower(fmt.Sprintf("%s_%s", strings.ReplaceAll(r.MetricName, " ", "_"), "Average")))
	dpAverage := metricAverage.SetEmptyGauge().DataPoints().AppendEmpty()
	dpAverage.SetStartTimestamp(startTimestamp)
	dpAverage.SetTimestamp(nanos)
	dpAverage.SetDoubleValue(r.Average)

	return nil
}

// azureMetricRecord represents a single Azure Metric following
// the common schema does not exist (yet):
type azureAppMetricRecord struct {
	Time       string `json:"time"`
	ResourceID string `json:"resourceId"`
	Type       string `json:"Type"`

	AppRoleInstance string `json:"AppRoleInstance"`
	AppRoleName     string `json:"AppRoleName"`
	AppVersion      string `json:"AppVersion"`
	SDKVersion      string `json:"SDKVersion"`

	ClientBrowser         string `json:"ClientBrowser"`
	ClientCity            string `json:"ClientCity"`
	ClientCountryOrRegion string `json:"ClientCountryOrRegion"`
	ClientIP              string `json:"ClientIP"`
	ClientModel           string `json:"ClientModel"`
	ClientOS              string `json:"ClientOS"`
	ClientStateOrProvince string `json:"ClientStateOrProvince"`
	ClientType            string `json:"ClientType"`

	MetricName string  `json:"Name"`
	Total      float64 `json:"Sum"`
	Minimum    float64 `json:"Min"`
	Maximum    float64 `json:"Max"`
	Count      float64 `json:"ItemCount"`
}

func (r *azureAppMetricRecord) AppendMetric(c azureResourceMetricsConfiger, md *pmetric.Metrics) error {
	resourceMetrics := md.ResourceMetrics().AppendEmpty()

	resource := resourceMetrics.Resource()
	resource.Attributes().PutStr(string(conventions.TelemetrySDKVersionKey), r.SDKVersion)
	resource.Attributes().PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderAzure.Value.AsString())
	resource.Attributes().PutStr(string(conventions.ServiceInstanceIDKey), r.AppRoleInstance)
	resource.Attributes().PutStr(string(conventions.ServiceNameKey), r.AppRoleName)
	resource.Attributes().PutStr(string(conventions.ServiceVersionKey), r.AppVersion)

	scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()

	metrics := scopeMetrics.Metrics()
	metrics.EnsureCapacity(4)

	if r.ResourceID != "" {
		resourceMetrics.Resource().Attributes().PutStr(azureResourceID, r.ResourceID)
	} else {
		c.GetLogger().Warn("No ResourceID Set on Metrics!")
	}

	nanos, err := asTimestamp(r.Time, c.GetTimeFormat())
	if err != nil {
		c.GetLogger().Warn("Invalid Timestamp", zap.String("time", r.Time))
		return err
	}

	startTimestamp := pcommon.NewTimestampFromTime(nanos.AsTime().Add(-time.Minute))

	metricTotal := metrics.AppendEmpty()
	metricTotal.SetName(strings.ToLower(fmt.Sprintf("%s_%s", strings.ReplaceAll(r.MetricName, " ", "_"), "Total")))
	dpTotal := metricTotal.SetEmptyGauge().DataPoints().AppendEmpty()
	dpTotal.SetStartTimestamp(startTimestamp)
	dpTotal.SetTimestamp(nanos)
	dpTotal.SetDoubleValue(r.Total)

	metricCount := metrics.AppendEmpty()
	metricCount.SetName(strings.ToLower(fmt.Sprintf("%s_%s", strings.ReplaceAll(r.MetricName, " ", "_"), "Count")))
	dpCount := metricCount.SetEmptyGauge().DataPoints().AppendEmpty()
	dpCount.SetStartTimestamp(startTimestamp)
	dpCount.SetTimestamp(nanos)
	dpCount.SetDoubleValue(r.Count)

	metricMin := metrics.AppendEmpty()
	metricMin.SetName(strings.ToLower(fmt.Sprintf("%s_%s", strings.ReplaceAll(r.MetricName, " ", "_"), "Minimum")))
	dpMin := metricMin.SetEmptyGauge().DataPoints().AppendEmpty()
	dpMin.SetStartTimestamp(startTimestamp)
	dpMin.SetTimestamp(nanos)
	dpMin.SetDoubleValue(r.Minimum)

	metricMax := metrics.AppendEmpty()
	metricMax.SetName(strings.ToLower(fmt.Sprintf("%s_%s", strings.ReplaceAll(r.MetricName, " ", "_"), "Maximum")))
	dpMax := metricMax.SetEmptyGauge().DataPoints().AppendEmpty()
	dpMax.SetStartTimestamp(startTimestamp)
	dpMax.SetTimestamp(nanos)
	dpMax.SetDoubleValue(r.Maximum)

	return nil
}

func newAzureResourceMetricsUnmarshaler(buildInfo component.BuildInfo, logger *zap.Logger, timeFormat []string) eventMetricsUnmarshaler {
	return &azureResourceMetricsUnmarshaler{
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
func (r *azureResourceMetricsUnmarshaler) UnmarshalMetrics(event *azureEvent) (pmetric.Metrics, error) {
	md := pmetric.NewMetrics()

	var azureMetrics azureMetricRecords
	decoder := jsoniter.NewDecoder(bytes.NewReader(event.Data()))
	err := decoder.Decode(&azureMetrics)

	if err != nil {
		return md, err
	}

	for _, mr := range azureMetrics.Records {
		err := mr.Record.AppendMetric(r, &md)
		if err != nil {
			r.logger.Warn("Failed to append metric", zap.Error(err))
		}
	}

	return md, nil
}

func (r *azureResourceMetricsUnmarshaler) GetLogger() *zap.Logger {
	return r.logger
}

func (r *azureResourceMetricsUnmarshaler) GetBuildVersion() string {
	return r.buildInfo.Version
}

func (r *azureResourceMetricsUnmarshaler) GetTimeFormat() []string {
	return r.TimeFormat
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
