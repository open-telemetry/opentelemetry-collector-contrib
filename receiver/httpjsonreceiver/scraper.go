package httpjsonreceiver // import "httpjsonreceiver"

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tidwall/gjson"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type Scraper struct {
	cfg    *Config
	client *http.Client
	logger *zap.Logger
}

func NewScraper(cfg *Config, client *http.Client, logger *zap.Logger) *Scraper {
	return &Scraper{
		cfg:    cfg,
		client: client,
		logger: logger,
	}
}

func (s *Scraper) Scrape(ctx context.Context) (pmetric.Metrics, error) {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()

	resource := rm.Resource()
	resource.Attributes().PutStr("receiver", "httpjson")

	for key, value := range s.cfg.ResourceAttributes {
		resource.Attributes().PutStr(key, value)
	}

	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("httpjsonreceiver")
	sm.Scope().SetVersion("1.0.0")

	for _, endpoint := range s.cfg.Endpoints {
		if err := s.scrapeEndpoint(ctx, endpoint, sm); err != nil {
			s.logger.Error("Failed to scrape endpoint",
				zap.String("url", endpoint.URL),
				zap.Error(err))
		}
	}

	return metrics, nil
}

func (s *Scraper) scrapeEndpoint(ctx context.Context, endpoint EndpointConfig, sm pmetric.ScopeMetrics) error {
	start := time.Now()

	req, err := http.NewRequestWithContext(ctx, endpoint.Method, endpoint.URL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	for key, value := range endpoint.Headers {
		req.Header.Set(key, value)
	}

	if endpoint.Body != "" && (endpoint.Method == "POST" || endpoint.Method == "PUT") {
		req.Body = io.NopCloser(strings.NewReader(endpoint.Body))
		if req.Header.Get("Content-Type") == "" {
			req.Header.Set("Content-Type", "application/json")
		}
	}

	if endpoint.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, endpoint.Timeout)
		defer cancel()
		req = req.WithContext(ctx)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	duration := time.Since(start)
	s.logger.Debug("HTTP request completed",
		zap.String("url", endpoint.URL),
		zap.Int("status", resp.StatusCode),
		zap.Duration("duration", duration))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP request failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	return s.parseAndEmitMetrics(body, endpoint, sm)
}

func (s *Scraper) parseAndEmitMetrics(jsonData []byte, endpoint EndpointConfig, sm pmetric.ScopeMetrics) error {
	if !gjson.ValidBytes(jsonData) {
		return fmt.Errorf("invalid JSON response")
	}

	for _, metricCfg := range endpoint.Metrics {
		if err := s.extractAndEmitMetric(jsonData, metricCfg, endpoint, sm); err != nil {
			s.logger.Warn("Failed to extract metric",
				zap.String("metric", metricCfg.Name),
				zap.String("path", metricCfg.JSONPath),
				zap.String("url", endpoint.URL),
				zap.Error(err))
		}
	}

	return nil
}

func (s *Scraper) extractAndEmitMetric(jsonData []byte, metricCfg MetricConfig, endpoint EndpointConfig, sm pmetric.ScopeMetrics) error {
	result := gjson.GetBytes(jsonData, metricCfg.JSONPath)
	if !result.Exists() {
		return fmt.Errorf("JSONPath %q not found", metricCfg.JSONPath)
	}

	var intValue int64
	var floatValue float64
	var err error

	switch result.Type {
	case gjson.Number:
		floatValue = result.Float()
		intValue = result.Int()
	case gjson.String:
		if metricCfg.ValueType == "int" {
			intValue, err = strconv.ParseInt(result.String(), 10, 64)
			floatValue = float64(intValue)
		} else {
			floatValue, err = strconv.ParseFloat(result.String(), 64)
			intValue = int64(floatValue)
		}
	case gjson.True:
		intValue = 1
		floatValue = 1.0
	case gjson.False:
		intValue = 0
		floatValue = 0.0
	default:
		return fmt.Errorf("cannot convert %s to numeric value", result.Type)
	}

	if err != nil {
		return fmt.Errorf("failed to parse value: %w", err)
	}

	metric := sm.Metrics().AppendEmpty()
	metric.SetName(metricCfg.Name)
	if metricCfg.Description != "" {
		metric.SetDescription(metricCfg.Description)
	}
	if metricCfg.Unit != "" {
		metric.SetUnit(metricCfg.Unit)
	}

	attrs := pcommon.NewMap()
	attrs.PutStr("http.url", endpoint.URL)
	attrs.PutStr("http.method", endpoint.Method)
	if endpoint.Name != "" {
		attrs.PutStr("endpoint.name", endpoint.Name)
	}
	attrs.PutStr("json.path", metricCfg.JSONPath)

	for k, v := range metricCfg.Attributes {
		attrs.PutStr(k, v)
	}

	now := pcommon.NewTimestampFromTime(time.Now())

	switch metricCfg.Type {
	case "gauge":
		gauge := metric.SetEmptyGauge()
		dp := gauge.DataPoints().AppendEmpty()
		dp.SetTimestamp(now)
		if metricCfg.ValueType == "int" {
			dp.SetIntValue(intValue)
		} else {
			dp.SetDoubleValue(floatValue)
		}
		attrs.CopyTo(dp.Attributes())

	case "counter":
		sum := metric.SetEmptySum()
		sum.SetIsMonotonic(true)
		sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		dp := sum.DataPoints().AppendEmpty()
		dp.SetTimestamp(now)
		if metricCfg.ValueType == "int" {
			dp.SetIntValue(intValue)
		} else {
			dp.SetDoubleValue(floatValue)
		}
		attrs.CopyTo(dp.Attributes())

	case "histogram":
		histogram := metric.SetEmptyHistogram()
		histogram.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		dp := histogram.DataPoints().AppendEmpty()
		dp.SetTimestamp(now)
		dp.SetCount(1)
		dp.SetSum(floatValue)
		dp.BucketCounts().Append(1)
		dp.ExplicitBounds().Append(floatValue)
		attrs.CopyTo(dp.Attributes())
	}

	s.logger.Debug("Extracted metric",
		zap.String("name", metricCfg.Name),
		zap.Float64("value", floatValue),
		zap.String("type", metricCfg.Type))

	return nil
}
