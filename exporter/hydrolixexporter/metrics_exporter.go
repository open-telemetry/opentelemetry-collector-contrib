package hydrolixexporter

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type metricsExporter struct {
	config *Config
	client *http.Client
	logger *zap.Logger
}

type HydrolixMetric struct {
	Name                   string                   `json:"name"`
	Description            string                   `json:"description,omitempty"`
	Unit                   string                   `json:"unit,omitempty"`
	MetricType             string                   `json:"metric_type"`
	Timestamp              uint64                   `json:"timestamp"`
	StartTime              uint64                   `json:"start_time,omitempty"`
	Count                  uint64                   `json:"count,omitempty"`
	Value                  float64                  `json:"value,omitempty"`
	BucketCounts           []uint64                 `json:"bucket_counts,omitempty"`
	ExplicitBounds         []float64                `json:"explicit_bounds,omitempty"`
	Min                    float64                  `json:"min,omitempty"`
	Max                    float64                  `json:"max,omitempty"`
	Sum                    float64                  `json:"sum,omitempty"`
	MetricAttributes       []map[string]interface{} `json:"tags"`
	ResourceAttributes     []map[string]interface{} `json:"serviceTags"`
	ResourceSchemaUrl      string                   `json:"resource_schema_url,omitempty"`
	ScopeName              string                   `json:"scope_name,omitempty"`
	ScopeVersion           string                   `json:"scope_version,omitempty"`
	ScopeAttributes        []map[string]interface{} `json:"scope_attributes,omitempty"`
	ScopeDroppedAttrCount  uint32                   `json:"scope_dropped_attr_count,omitempty"`
	ScopeSchemaUrl         string                   `json:"scope_schema_url,omitempty"`
	Scale                  int32                    `json:"scale,omitempty"`
	ZeroCount              uint64                   `json:"zero_count,omitempty"`
	Positive               string                   `json:"positive,omitempty"`
	Negative               string                   `json:"negative,omitempty"`
	Exemplars              []Exemplar               `json:"exemplars,omitempty"`
	Flags                  uint32                   `json:"flags,omitempty"`
	AggregationTemporality int32                    `json:"aggregation_temporality,omitempty"`
	IsMonotonic            bool                     `json:"is_monotonic,omitempty"`
	Quantiles              []float64                `json:"quantiles,omitempty"`
	QuantileValues         []float64                `json:"quantile_values,omitempty"`
	ServiceName            string                   `json:"serviceName,omitempty"`
	HTTPStatusCode         string                   `json:"httpStatusCode,omitempty"`
	HTTPRoute              string                   `json:"httpRoute,omitempty"`
	HTTPMethod             string                   `json:"httpMethod,omitempty"`
	TraceID                string                   `json:"traceId,omitempty"`
	SpanID                 string                   `json:"spanId,omitempty"`
}

type Exemplar struct {
	FilteredAttributes map[string]string `json:"filtered_attributes"`
	Timestamp          uint64            `json:"timestamp"`
	Value              float64           `json:"value"`
	SpanID             string            `json:"span_id"`
	TraceID            string            `json:"trace_id"`
}

func newMetricsExporter(config *Config, set exporter.Settings) *metricsExporter {
	return &metricsExporter{
		config: config,
		client: &http.Client{Timeout: config.Timeout},
		logger: set.Logger,
	}
}

func (e *metricsExporter) pushMetrics(ctx context.Context, md pmetric.Metrics) error {
	metrics := e.convertToHydrolixMetrics(md)

	jsonData, err := json.Marshal(metrics)
	if err != nil {
		return fmt.Errorf("failed to marshal metrics: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", e.config.Endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-hdx-table", e.config.HDXTable)
	req.Header.Set("x-hdx-transform", e.config.HDXTransform)
	req.SetBasicAuth(e.config.HDXUsername, e.config.HDXPassword)

	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to Hydrolix: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return fmt.Errorf("unexpected status code: %d (failed to read response body: %v)", resp.StatusCode, readErr)
		}
		return fmt.Errorf("unexpected status code: %d, response: %s", resp.StatusCode, string(body))
	}

	e.logger.Debug("successfully sent metrics to Hydrolix",
		zap.Int("metric_count", len(metrics)),
		zap.String("table", e.config.HDXTable))

	return nil
}

func (e *metricsExporter) convertToHydrolixMetrics(md pmetric.Metrics) []HydrolixMetric {
	var metrics []HydrolixMetric

	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		resource := rm.Resource()
		resourceAttrs := convertAttributes(resource.Attributes())
		resourceSchemaUrl := rm.SchemaUrl()

		serviceName := extractStringAttr(resource.Attributes(), "service.name")

		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			scope := sm.Scope()
			scopeAttrs := convertAttributes(scope.Attributes())

			for k := 0; k < sm.Metrics().Len(); k++ {
				metric := sm.Metrics().At(k)

				switch metric.Type() {
				case pmetric.MetricTypeGauge:
					metrics = append(metrics, e.convertGauge(metric, resourceAttrs, resourceSchemaUrl, scopeAttrs, scope, sm.SchemaUrl(), serviceName)...)
				case pmetric.MetricTypeSum:
					metrics = append(metrics, e.convertSum(metric, resourceAttrs, resourceSchemaUrl, scopeAttrs, scope, sm.SchemaUrl(), serviceName)...)
				case pmetric.MetricTypeHistogram:
					metrics = append(metrics, e.convertHistogram(metric, resourceAttrs, resourceSchemaUrl, scopeAttrs, scope, sm.SchemaUrl(), serviceName)...)
				case pmetric.MetricTypeExponentialHistogram:
					metrics = append(metrics, e.convertExponentialHistogram(metric, resourceAttrs, resourceSchemaUrl, scopeAttrs, scope, sm.SchemaUrl(), serviceName)...)
				case pmetric.MetricTypeSummary:
					metrics = append(metrics, e.convertSummary(metric, resourceAttrs, resourceSchemaUrl, scopeAttrs, scope, sm.SchemaUrl(), serviceName)...)
				}
			}
		}
	}

	return metrics
}

func (e *metricsExporter) convertGauge(metric pmetric.Metric, resourceAttrs []map[string]interface{}, resourceSchemaUrl string, scopeAttrs []map[string]interface{}, scope pcommon.InstrumentationScope, scopeSchemaUrl, serviceName string) []HydrolixMetric {
	var metrics []HydrolixMetric

	gauge := metric.Gauge()
	for i := 0; i < gauge.DataPoints().Len(); i++ {
		dp := gauge.DataPoints().At(i)

		hdxMetric := HydrolixMetric{
			Name:                  metric.Name(),
			Description:           metric.Description(),
			Unit:                  metric.Unit(),
			MetricType:            "gauge",
			Timestamp:             uint64(dp.Timestamp()),
			StartTime:             uint64(dp.StartTimestamp()),
			MetricAttributes:      convertAttributes(dp.Attributes()),
			ResourceAttributes:    resourceAttrs,
			ResourceSchemaUrl:     resourceSchemaUrl,
			ScopeName:             scope.Name(),
			ScopeVersion:          scope.Version(),
			ScopeAttributes:       scopeAttrs,
			ScopeDroppedAttrCount: scope.DroppedAttributesCount(),
			ScopeSchemaUrl:        scopeSchemaUrl,
			Exemplars:             convertExemplars(dp.Exemplars()),
			Flags:                 uint32(dp.Flags()),
			ServiceName:           serviceName,
			HTTPStatusCode:        extractStringAttr(dp.Attributes(), "http.response.status_code"),
			HTTPRoute:             extractStringAttr(dp.Attributes(), "http.route"),
			HTTPMethod:            extractStringAttr(dp.Attributes(), "http.request.method"),
			TraceID:               extractStringAttr(dp.Attributes(), "trace_id"),
			SpanID:                extractStringAttr(dp.Attributes(), "span_id"),
		}

		switch dp.ValueType() {
		case pmetric.NumberDataPointValueTypeInt:
			hdxMetric.Value = float64(dp.IntValue())
		case pmetric.NumberDataPointValueTypeDouble:
			hdxMetric.Value = dp.DoubleValue()
		}

		metrics = append(metrics, hdxMetric)
	}

	return metrics
}

func (e *metricsExporter) convertSum(metric pmetric.Metric, resourceAttrs []map[string]interface{}, resourceSchemaUrl string, scopeAttrs []map[string]interface{}, scope pcommon.InstrumentationScope, scopeSchemaUrl, serviceName string) []HydrolixMetric {
	var metrics []HydrolixMetric

	sum := metric.Sum()
	for i := 0; i < sum.DataPoints().Len(); i++ {
		dp := sum.DataPoints().At(i)

		hdxMetric := HydrolixMetric{
			Name:                   metric.Name(),
			Description:            metric.Description(),
			Unit:                   metric.Unit(),
			MetricType:             "sum",
			Timestamp:              uint64(dp.Timestamp()),
			StartTime:              uint64(dp.StartTimestamp()),
			MetricAttributes:       convertAttributes(dp.Attributes()),
			ResourceAttributes:     resourceAttrs,
			ResourceSchemaUrl:      resourceSchemaUrl,
			ScopeName:              scope.Name(),
			ScopeVersion:           scope.Version(),
			ScopeAttributes:        scopeAttrs,
			ScopeDroppedAttrCount:  scope.DroppedAttributesCount(),
			ScopeSchemaUrl:         scopeSchemaUrl,
			Exemplars:              convertExemplars(dp.Exemplars()),
			Flags:                  uint32(dp.Flags()),
			AggregationTemporality: int32(sum.AggregationTemporality()),
			IsMonotonic:            sum.IsMonotonic(),
			ServiceName:            serviceName,
			HTTPStatusCode:         extractStringAttr(dp.Attributes(), "http.response.status_code"),
			HTTPRoute:              extractStringAttr(dp.Attributes(), "http.route"),
			HTTPMethod:             extractStringAttr(dp.Attributes(), "http.request.method"),
			TraceID:                extractStringAttr(dp.Attributes(), "trace_id"),
			SpanID:                 extractStringAttr(dp.Attributes(), "span_id"),
		}

		switch dp.ValueType() {
		case pmetric.NumberDataPointValueTypeInt:
			hdxMetric.Value = float64(dp.IntValue())
		case pmetric.NumberDataPointValueTypeDouble:
			hdxMetric.Value = dp.DoubleValue()
		}

		metrics = append(metrics, hdxMetric)
	}

	return metrics
}

func (e *metricsExporter) convertHistogram(metric pmetric.Metric, resourceAttrs []map[string]interface{}, resourceSchemaUrl string, scopeAttrs []map[string]interface{}, scope pcommon.InstrumentationScope, scopeSchemaUrl, serviceName string) []HydrolixMetric {
	var metrics []HydrolixMetric

	histogram := metric.Histogram()
	for i := 0; i < histogram.DataPoints().Len(); i++ {
		dp := histogram.DataPoints().At(i)

		hdxMetric := HydrolixMetric{
			Name:                   metric.Name(),
			Description:            metric.Description(),
			Unit:                   metric.Unit(),
			MetricType:             "histogram",
			Timestamp:              uint64(dp.Timestamp()),
			StartTime:              uint64(dp.StartTimestamp()),
			Count:                  dp.Count(),
			Sum:                    dp.Sum(),
			Min:                    dp.Min(),
			Max:                    dp.Max(),
			BucketCounts:           convertBucketCounts(dp.BucketCounts()),
			ExplicitBounds:         convertExplicitBounds(dp.ExplicitBounds()),
			MetricAttributes:       convertAttributes(dp.Attributes()),
			ResourceAttributes:     resourceAttrs,
			ResourceSchemaUrl:      resourceSchemaUrl,
			ScopeName:              scope.Name(),
			ScopeVersion:           scope.Version(),
			ScopeAttributes:        scopeAttrs,
			ScopeDroppedAttrCount:  scope.DroppedAttributesCount(),
			ScopeSchemaUrl:         scopeSchemaUrl,
			Exemplars:              convertExemplars(dp.Exemplars()),
			Flags:                  uint32(dp.Flags()),
			AggregationTemporality: int32(histogram.AggregationTemporality()),
			ServiceName:            serviceName,
			HTTPStatusCode:         extractStringAttr(dp.Attributes(), "http.response.status_code"),
			HTTPRoute:              extractStringAttr(dp.Attributes(), "http.route"),
			HTTPMethod:             extractStringAttr(dp.Attributes(), "http.request.method"),
			TraceID:                extractStringAttr(dp.Attributes(), "trace_id"),
			SpanID:                 extractStringAttr(dp.Attributes(), "span_id"),
		}

		metrics = append(metrics, hdxMetric)
	}

	return metrics
}

func (e *metricsExporter) convertExponentialHistogram(metric pmetric.Metric, resourceAttrs []map[string]interface{}, resourceSchemaUrl string, scopeAttrs []map[string]interface{}, scope pcommon.InstrumentationScope, scopeSchemaUrl, serviceName string) []HydrolixMetric {
	var metrics []HydrolixMetric

	expHistogram := metric.ExponentialHistogram()
	for i := 0; i < expHistogram.DataPoints().Len(); i++ {
		dp := expHistogram.DataPoints().At(i)

		positive, _ := json.Marshal(map[string]interface{}{
			"offset":        dp.Positive().Offset(),
			"bucket_counts": convertBucketCounts(dp.Positive().BucketCounts()),
		})

		negative, _ := json.Marshal(map[string]interface{}{
			"offset":        dp.Negative().Offset(),
			"bucket_counts": convertBucketCounts(dp.Negative().BucketCounts()),
		})

		hdxMetric := HydrolixMetric{
			Name:                   metric.Name(),
			Description:            metric.Description(),
			Unit:                   metric.Unit(),
			MetricType:             "exponentialHistogram",
			Timestamp:              uint64(dp.Timestamp()),
			StartTime:              uint64(dp.StartTimestamp()),
			Count:                  dp.Count(),
			Sum:                    dp.Sum(),
			Min:                    dp.Min(),
			Max:                    dp.Max(),
			Scale:                  dp.Scale(),
			ZeroCount:              dp.ZeroCount(),
			Positive:               string(positive),
			Negative:               string(negative),
			MetricAttributes:       convertAttributes(dp.Attributes()),
			ResourceAttributes:     resourceAttrs,
			ResourceSchemaUrl:      resourceSchemaUrl,
			ScopeName:              scope.Name(),
			ScopeVersion:           scope.Version(),
			ScopeAttributes:        scopeAttrs,
			ScopeDroppedAttrCount:  scope.DroppedAttributesCount(),
			ScopeSchemaUrl:         scopeSchemaUrl,
			Exemplars:              convertExemplars(dp.Exemplars()),
			Flags:                  uint32(dp.Flags()),
			AggregationTemporality: int32(expHistogram.AggregationTemporality()),
			ServiceName:            serviceName,
			HTTPStatusCode:         extractStringAttr(dp.Attributes(), "http.response.status_code"),
			HTTPRoute:              extractStringAttr(dp.Attributes(), "http.route"),
			HTTPMethod:             extractStringAttr(dp.Attributes(), "http.request.method"),
			TraceID:                extractStringAttr(dp.Attributes(), "trace_id"),
			SpanID:                 extractStringAttr(dp.Attributes(), "span_id"),
		}

		metrics = append(metrics, hdxMetric)
	}

	return metrics
}

func (e *metricsExporter) convertSummary(metric pmetric.Metric, resourceAttrs []map[string]interface{}, resourceSchemaUrl string, scopeAttrs []map[string]interface{}, scope pcommon.InstrumentationScope, scopeSchemaUrl, serviceName string) []HydrolixMetric {
	var metrics []HydrolixMetric

	summary := metric.Summary()
	for i := 0; i < summary.DataPoints().Len(); i++ {
		dp := summary.DataPoints().At(i)

		// Convert quantiles
		quantiles := make([]float64, dp.QuantileValues().Len())
		quantileValues := make([]float64, dp.QuantileValues().Len())
		for j := 0; j < dp.QuantileValues().Len(); j++ {
			qv := dp.QuantileValues().At(j)
			quantiles[j] = qv.Quantile()
			quantileValues[j] = qv.Value()
		}

		hdxMetric := HydrolixMetric{
			Name:                  metric.Name(),
			Description:           metric.Description(),
			Unit:                  metric.Unit(),
			MetricType:            "summary",
			Timestamp:             uint64(dp.Timestamp()),
			StartTime:             uint64(dp.StartTimestamp()),
			Count:                 dp.Count(),
			Sum:                   dp.Sum(),
			Quantiles:             quantiles,
			QuantileValues:        quantileValues,
			MetricAttributes:      convertAttributes(dp.Attributes()),
			ResourceAttributes:    resourceAttrs,
			ResourceSchemaUrl:     resourceSchemaUrl,
			ScopeName:             scope.Name(),
			ScopeVersion:          scope.Version(),
			ScopeAttributes:       scopeAttrs,
			ScopeDroppedAttrCount: scope.DroppedAttributesCount(),
			ScopeSchemaUrl:        scopeSchemaUrl,
			Flags:                 uint32(dp.Flags()),
			ServiceName:           serviceName,
			HTTPStatusCode:        extractStringAttr(dp.Attributes(), "http.response.status_code"),
			HTTPRoute:             extractStringAttr(dp.Attributes(), "http.route"),
			HTTPMethod:            extractStringAttr(dp.Attributes(), "http.request.method"),
			TraceID:               extractStringAttr(dp.Attributes(), "trace_id"),
			SpanID:                extractStringAttr(dp.Attributes(), "span_id"),
		}

		metrics = append(metrics, hdxMetric)
	}

	return metrics
}

func convertExemplars(exemplars pmetric.ExemplarSlice) []Exemplar {
	if exemplars.Len() == 0 {
		return nil
	}

	result := make([]Exemplar, 0, exemplars.Len())
	for i := 0; i < exemplars.Len(); i++ {
		exemplar := exemplars.At(i)

		// Convert filtered attributes
		attrs := make(map[string]string)
		exemplar.FilteredAttributes().Range(func(k string, v pcommon.Value) bool {
			attrs[k] = v.AsString()
			return true
		})

		// Get value based on type
		var value float64
		switch exemplar.ValueType() {
		case pmetric.ExemplarValueTypeDouble:
			value = exemplar.DoubleValue()
		case pmetric.ExemplarValueTypeInt:
			value = float64(exemplar.IntValue())
		}

		// Convert trace and span IDs to hex strings
		traceID := exemplar.TraceID()
		spanID := exemplar.SpanID()

		result = append(result, Exemplar{
			FilteredAttributes: attrs,
			Timestamp:          uint64(exemplar.Timestamp()),
			Value:              value,
			SpanID:             hex.EncodeToString(spanID[:]),
			TraceID:            hex.EncodeToString(traceID[:]),
		})
	}

	return result
}

func convertBucketCounts(bc pcommon.UInt64Slice) []uint64 {
	counts := make([]uint64, bc.Len())
	for i := 0; i < bc.Len(); i++ {
		counts[i] = bc.At(i)
	}
	return counts
}

func convertExplicitBounds(eb pcommon.Float64Slice) []float64 {
	bounds := make([]float64, eb.Len())
	for i := 0; i < eb.Len(); i++ {
		bounds[i] = eb.At(i)
	}
	return bounds
}
