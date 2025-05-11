// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dorisexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dorisexporter"

import (
	"context"
	_ "embed" // for SQL file embedding
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	semconv "go.opentelemetry.io/otel/semconv/v1.25.0"
	"go.uber.org/zap"
)

var ddls = []string{
	metricsGaugeDDL,
	metricsSumDDL,
	metricsHistogramDDL,
	metricsExponentialHistogramDDL,
	metricsSummaryDDL,
}

//go:embed sql/metrics_view.sql
var metricsView string

type metricsExporter struct {
	*commonExporter
}

func newMetricsExporter(logger *zap.Logger, cfg *Config, set component.TelemetrySettings) *metricsExporter {
	return &metricsExporter{
		commonExporter: newExporter(logger, cfg, set, "METRIC"),
	}
}

func (e *metricsExporter) start(ctx context.Context, host component.Host) error {
	client, err := createDorisHTTPClient(ctx, e.cfg, host, e.TelemetrySettings)
	if err != nil {
		return err
	}
	e.client = client

	if e.cfg.CreateSchema {
		conn, err := createDorisMySQLClient(e.cfg)
		if err != nil {
			return err
		}
		defer conn.Close()

		err = createAndUseDatabase(ctx, conn, e.cfg)
		if err != nil {
			return err
		}

		for _, ddlTemplate := range ddls {
			ddl := fmt.Sprintf(ddlTemplate, e.cfg.Metrics, e.cfg.propertiesStr())
			_, err = conn.ExecContext(ctx, ddl)
			if err != nil {
				return err
			}
		}

		models := []metricModel{
			&metricModelGauge{},
			&metricModelSum{},
			&metricModelHistogram{},
			&metricModelExponentialHistogram{},
			&metricModelSummary{},
		}

		for _, model := range models {
			table := e.cfg.Metrics + model.tableSuffix()
			view := fmt.Sprintf(metricsView, table, table)
			_, err = conn.ExecContext(ctx, view)
			if err != nil {
				e.logger.Warn("failed to create materialized view", zap.Error(err))
			}
		}
	}

	go e.reporter.report()
	return nil
}

func (e *metricsExporter) shutdown(_ context.Context) error {
	if e.client != nil {
		e.client.CloseIdleConnections()
	}
	return nil
}

func (e *metricsExporter) initMetricMap(ms pmetric.Metrics) map[pmetric.MetricType]metricModel {
	metricMap := make(map[pmetric.MetricType]metricModel, 5)

	gaugeLen := 0
	sumLen := 0
	histogramLen := 0
	exponentialHistogramLen := 0
	summaryLen := 0

	rms := ms.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		ilms := rm.ScopeMetrics()
		for j := 0; j < ilms.Len(); j++ {
			ilm := ilms.At(j)
			ms := ilm.Metrics()
			for k := 0; k < ms.Len(); k++ {
				m := ms.At(k)
				switch m.Type() {
				case pmetric.MetricTypeGauge:
					gaugeLen += m.Gauge().DataPoints().Len()
				case pmetric.MetricTypeSum:
					sumLen += m.Sum().DataPoints().Len()
				case pmetric.MetricTypeHistogram:
					histogramLen += m.Histogram().DataPoints().Len()
				case pmetric.MetricTypeExponentialHistogram:
					exponentialHistogramLen += m.ExponentialHistogram().DataPoints().Len()
				case pmetric.MetricTypeSummary:
					summaryLen += m.Summary().DataPoints().Len()
				}
			}
		}
	}

	if gaugeLen > 0 {
		gauge := &metricModelGauge{}
		gauge.data = make([]*dMetricGauge, 0, gaugeLen)
		gauge.lbl = e.generateMetricLabel(gauge)
		metricMap[pmetric.MetricTypeGauge] = gauge
	}

	if sumLen > 0 {
		sum := &metricModelSum{}
		sum.data = make([]*dMetricSum, 0, sumLen)
		sum.lbl = e.generateMetricLabel(sum)
		metricMap[pmetric.MetricTypeSum] = sum
	}

	if histogramLen > 0 {
		histogram := &metricModelHistogram{}
		histogram.data = make([]*dMetricHistogram, 0, histogramLen)
		histogram.lbl = e.generateMetricLabel(histogram)
		metricMap[pmetric.MetricTypeHistogram] = histogram
	}

	if exponentialHistogramLen > 0 {
		exponentialHistogram := &metricModelExponentialHistogram{}
		exponentialHistogram.data = make([]*dMetricExponentialHistogram, 0, exponentialHistogramLen)
		exponentialHistogram.lbl = e.generateMetricLabel(exponentialHistogram)
		metricMap[pmetric.MetricTypeExponentialHistogram] = exponentialHistogram
	}

	if summaryLen > 0 {
		summary := &metricModelSummary{}
		summary.data = make([]*dMetricSummary, 0, summaryLen)
		summary.lbl = e.generateMetricLabel(summary)
		metricMap[pmetric.MetricTypeSummary] = summary
	}

	return metricMap
}

func (e *metricsExporter) pushMetricData(ctx context.Context, md pmetric.Metrics) error {
	metricMap := e.initMetricMap(md)

	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		resourceMetric := md.ResourceMetrics().At(i)
		resource := resourceMetric.Resource()
		resourceAttributes := resource.Attributes()
		serviceName := ""
		v, ok := resourceAttributes.Get(string(semconv.ServiceNameKey))
		if ok {
			serviceName = v.AsString()
		}
		serviceInstance := ""
		v, ok = resourceAttributes.Get(string(semconv.ServiceInstanceIDKey))
		if ok {
			serviceInstance = v.AsString()
		}

		for j := 0; j < resourceMetric.ScopeMetrics().Len(); j++ {
			scopeMetric := resourceMetric.ScopeMetrics().At(j)

			for k := 0; k < scopeMetric.Metrics().Len(); k++ {
				metric := scopeMetric.Metrics().At(k)

				dm := &dMetric{
					ServiceName:        serviceName,
					ServiceInstanceID:  serviceInstance,
					MetricName:         metric.Name(),
					MetricDescription:  metric.Description(),
					MetricUnit:         metric.Unit(),
					ResourceAttributes: resourceAttributes.AsRaw(),
					ScopeName:          scopeMetric.Scope().Name(),
					ScopeVersion:       scopeMetric.Scope().Version(),
				}

				metricM, ok := metricMap[metric.Type()]
				if !ok {
					return fmt.Errorf("invalid metric type: %v", metric.Type().String())
				}

				err := metricM.add(metric, dm, e)
				if err != nil {
					return err
				}
			}
		}
	}

	return e.pushMetricDataParallel(ctx, metricMap)
}

func (e *metricsExporter) pushMetricDataParallel(ctx context.Context, metricMap map[pmetric.MetricType]metricModel) error {
	errChan := make(chan error, len(metricMap))
	wg := &sync.WaitGroup{}
	for _, m := range metricMap {
		if m.size() <= 0 {
			continue
		}

		wg.Add(1)
		go func(m metricModel, wg *sync.WaitGroup) {
			errChan <- e.pushMetricDataInternal(ctx, m)
			wg.Done()
		}(m, wg)
	}
	wg.Wait()
	close(errChan)
	var errs error
	for err := range errChan {
		errs = errors.Join(errs, err)
	}
	return errs
}

func (e *metricsExporter) pushMetricDataInternal(ctx context.Context, metrics metricModel) error {
	marshal, err := metrics.bytes()
	if err != nil {
		return err
	}

	req, err := streamLoadRequest(ctx, e.cfg, e.cfg.Metrics+metrics.tableSuffix(), marshal, metrics.label())
	if err != nil {
		return err
	}

	res, err := e.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}

	response := streamLoadResponse{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return err
	}

	if response.success() {
		e.reporter.incrTotalRows(int64(metrics.size()))
		e.reporter.incrTotalBytes(int64(len(marshal)))

		if response.duplication() {
			e.logger.Warn("label already exists", zap.String("label", metrics.label()), zap.Int("skipped", metrics.size()))
		}

		if e.cfg.LogResponse {
			e.logger.Info("metric response:\n" + string(body))
		} else {
			e.logger.Debug("metric response:\n" + string(body))
		}
		return nil
	}

	return fmt.Errorf("failed to push metric data, response:%s", string(body))
}

func (e *metricsExporter) getNumberDataPointValue(dp pmetric.NumberDataPoint) float64 {
	switch dp.ValueType() {
	case pmetric.NumberDataPointValueTypeInt:
		return float64(dp.IntValue())
	case pmetric.NumberDataPointValueTypeDouble:
		return dp.DoubleValue()
	case pmetric.NumberDataPointValueTypeEmpty:
		e.logger.Warn("data point value type is unset, use 0.0 as default")
		return 0.0
	default:
		e.logger.Warn("data point value type is invalid, use 0.0 as default")
		return 0.0
	}
}

func (e *metricsExporter) getExemplarValue(ep pmetric.Exemplar) float64 {
	switch ep.ValueType() {
	case pmetric.ExemplarValueTypeInt:
		return float64(ep.IntValue())
	case pmetric.ExemplarValueTypeDouble:
		return ep.DoubleValue()
	case pmetric.ExemplarValueTypeEmpty:
		e.logger.Warn("exemplar value type is unset, use 0.0 as default")
		return 0.0
	default:
		e.logger.Warn("exemplar value type is invalid, use 0.0 as default")
		return 0.0
	}
}

func (e *metricsExporter) generateMetricLabel(m metricModel) string {
	return generateLabel(e.cfg, e.cfg.Metrics+m.tableSuffix())
}
