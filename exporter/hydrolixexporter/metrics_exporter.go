package hydrolixexporter

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
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
    Name               string     `json:"name"`
    Description        string     `json:"description,omitempty"`
    Unit               string     `json:"unit,omitempty"`
    MetricType         string     `json:"metric_type"`
    Timestamp          uint64     `json:"timestamp"`
    StartTime          uint64     `json:"start_time,omitempty"`
    Count              uint64     `json:"count,omitempty"`
    Value              float64    `json:"value,omitempty"`
    BucketCounts       []uint64   `json:"bucket_counts,omitempty"`
    ExplicitBounds     []float64  `json:"explicit_bounds,omitempty"`
    Min                float64    `json:"min,omitempty"`
    Max                float64    `json:"max,omitempty"`
    Sum                float64    `json:"sum,omitempty"`
    MetricAttributes   []TagValue `json:"tags"`
    ResourceAttributes []TagValue `json:"serviceTags"`
    Scale              int32      `json:"scale,omitempty"`
    Positive           string     `json:"positive,omitempty"`
    Negative           string     `json:"negative,omitempty"`
    ServiceName        string     `json:"serviceName,omitempty"`
    HTTPStatusCode     string     `json:"httpStatusCode,omitempty"`
    HTTPRoute          string     `json:"httpRoute,omitempty"`
    HTTPMethod         string     `json:"httpMethod,omitempty"`
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
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 300 {
        return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
    }

    return nil
}

func (e *metricsExporter) convertToHydrolixMetrics(md pmetric.Metrics) []HydrolixMetric {
    var metrics []HydrolixMetric

    for i := 0; i < md.ResourceMetrics().Len(); i++ {
        rm := md.ResourceMetrics().At(i)
        resource := rm.Resource()
        resourceAttrs := convertAttributes(resource.Attributes())

        serviceName := extractStringAttr(resource.Attributes(), "service.name")

        for j := 0; j < rm.ScopeMetrics().Len(); j++ {
            sm := rm.ScopeMetrics().At(j)

            for k := 0; k < sm.Metrics().Len(); k++ {
                metric := sm.Metrics().At(k)

                switch metric.Type() {
                case pmetric.MetricTypeGauge:
                    metrics = append(metrics, e.convertGauge(metric, resourceAttrs, serviceName)...)
                case pmetric.MetricTypeSum:
                    metrics = append(metrics, e.convertSum(metric, resourceAttrs, serviceName)...)
                case pmetric.MetricTypeHistogram:
                    metrics = append(metrics, e.convertHistogram(metric, resourceAttrs, serviceName)...)
                case pmetric.MetricTypeExponentialHistogram:
                    metrics = append(metrics, e.convertExponentialHistogram(metric, resourceAttrs, serviceName)...)
                }
            }
        }
    }

    return metrics
}

func (e *metricsExporter) convertGauge(metric pmetric.Metric, resourceAttrs []TagValue, serviceName string) []HydrolixMetric {
    var metrics []HydrolixMetric

    gauge := metric.Gauge()
    for i := 0; i < gauge.DataPoints().Len(); i++ {
        dp := gauge.DataPoints().At(i)

        hdxMetric := HydrolixMetric{
            Name:               metric.Name(),
            Description:        metric.Description(),
            Unit:               metric.Unit(),
            MetricType:         "gauge",
            Timestamp:          uint64(dp.Timestamp()),
            MetricAttributes:   convertAttributes(dp.Attributes()),
            ResourceAttributes: resourceAttrs,
            ServiceName:        serviceName,
            HTTPStatusCode:     extractStringAttr(dp.Attributes(), "http.response.status_code"),
            HTTPRoute:          extractStringAttr(dp.Attributes(), "http.route"),
            HTTPMethod:         extractStringAttr(dp.Attributes(), "http.request.method"),
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

func (e *metricsExporter) convertSum(metric pmetric.Metric, resourceAttrs []TagValue, serviceName string) []HydrolixMetric {
    var metrics []HydrolixMetric

    sum := metric.Sum()
    for i := 0; i < sum.DataPoints().Len(); i++ {
        dp := sum.DataPoints().At(i)

        hdxMetric := HydrolixMetric{
            Name:               metric.Name(),
            Description:        metric.Description(),
            Unit:               metric.Unit(),
            MetricType:         "sum",
            Timestamp:          uint64(dp.Timestamp()),
            StartTime:          uint64(dp.StartTimestamp()),
            MetricAttributes:   convertAttributes(dp.Attributes()),
            ResourceAttributes: resourceAttrs,
            ServiceName:        serviceName,
            HTTPStatusCode:     extractStringAttr(dp.Attributes(), "http.response.status_code"),
            HTTPRoute:          extractStringAttr(dp.Attributes(), "http.route"),
            HTTPMethod:         extractStringAttr(dp.Attributes(), "http.request.method"),
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

func (e *metricsExporter) convertHistogram(metric pmetric.Metric, resourceAttrs []TagValue, serviceName string) []HydrolixMetric {
    var metrics []HydrolixMetric

    histogram := metric.Histogram()
    for i := 0; i < histogram.DataPoints().Len(); i++ {
        dp := histogram.DataPoints().At(i)

        hdxMetric := HydrolixMetric{
            Name:               metric.Name(),
            Description:        metric.Description(),
            Unit:               metric.Unit(),
            MetricType:         "histogram",
            Timestamp:          uint64(dp.Timestamp()),
            StartTime:          uint64(dp.StartTimestamp()),
            Count:              dp.Count(),
            Sum:                dp.Sum(),
            Min:                dp.Min(),
            Max:                dp.Max(),
            BucketCounts:       convertBucketCounts(dp.BucketCounts()),
            ExplicitBounds:     convertExplicitBounds(dp.ExplicitBounds()),
            MetricAttributes:   convertAttributes(dp.Attributes()),
            ResourceAttributes: resourceAttrs,
            ServiceName:        serviceName,
            HTTPStatusCode:     extractStringAttr(dp.Attributes(), "http.response.status_code"),
            HTTPRoute:          extractStringAttr(dp.Attributes(), "http.route"),
            HTTPMethod:         extractStringAttr(dp.Attributes(), "http.request.method"),
        }

        metrics = append(metrics, hdxMetric)
    }

    return metrics
}

func (e *metricsExporter) convertExponentialHistogram(metric pmetric.Metric, resourceAttrs []TagValue, serviceName string) []HydrolixMetric {
    var metrics []HydrolixMetric

    expHistogram := metric.ExponentialHistogram()
    for i := 0; i < expHistogram.DataPoints().Len(); i++ {
        dp := expHistogram.DataPoints().At(i)

        positive, _ := json.Marshal(map[string]interface{}{
            "offset":       dp.Positive().Offset(),
            "bucket_counts": convertBucketCounts(dp.Positive().BucketCounts()),
        })

        negative, _ := json.Marshal(map[string]interface{}{
            "offset":       dp.Negative().Offset(),
            "bucket_counts": convertBucketCounts(dp.Negative().BucketCounts()),
        })

        hdxMetric := HydrolixMetric{
            Name:               metric.Name(),
            Description:        metric.Description(),
            Unit:               metric.Unit(),
            MetricType:         "exponentialHistogram",
            Timestamp:          uint64(dp.Timestamp()),
            StartTime:          uint64(dp.StartTimestamp()),
            Count:              dp.Count(),
            Sum:                dp.Sum(),
            Min:                dp.Min(),
            Max:                dp.Max(),
            Scale:              dp.Scale(),
            Positive:           string(positive),
            Negative:           string(negative),
            MetricAttributes:   convertAttributes(dp.Attributes()),
            ResourceAttributes: resourceAttrs,
            ServiceName:        serviceName,
            HTTPStatusCode:     extractStringAttr(dp.Attributes(), "http.response.status_code"),
            HTTPRoute:          extractStringAttr(dp.Attributes(), "http.route"),
            HTTPMethod:         extractStringAttr(dp.Attributes(), "http.request.method"),
        }

        metrics = append(metrics, hdxMetric)
    }

    return metrics
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