// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dorisexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dorisexporter"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	semconv "go.opentelemetry.io/collector/semconv/v1.25.0"
	"go.uber.org/zap"
)

var ddls = []string{
	metricsGaugeDDL,
	metricsSumDDL,
	metricsHistogramDDL,
	metricsExponentialHistogramDDL,
	metricsSummaryDDL,
}

func initMetricMap(maxLen int) map[pmetric.MetricType]metricModel {
	return map[pmetric.MetricType]metricModel{
		pmetric.MetricTypeGauge: &metricModelGauge{
			data: make([]*dMetricGauge, 0, maxLen),
		},
		pmetric.MetricTypeSum: &metricModelSum{
			data: make([]*dMetricSum, 0, maxLen),
		},
		pmetric.MetricTypeHistogram: &metricModelHistogram{
			data: make([]*dMetricHistogram, 0, maxLen),
		},
		pmetric.MetricTypeExponentialHistogram: &metricModelExponentialHistogram{
			data: make([]*dMetricExponentialHistogram, 0, maxLen),
		},
		pmetric.MetricTypeSummary: &metricModelSummary{
			data: make([]*dMetricSummary, 0, maxLen),
		},
	}
}

type metricsExporter struct {
	*commonExporter
}

func newMetricsExporter(logger *zap.Logger, cfg *Config, set component.TelemetrySettings) *metricsExporter {
	return &metricsExporter{
		commonExporter: newExporter(logger, cfg, set),
	}
}

func (e *metricsExporter) start(ctx context.Context, host component.Host) error {
	client, err := createDorisHTTPClient(ctx, e.cfg, host, e.TelemetrySettings)
	if err != nil {
		return err
	}
	e.client = client

	if !e.cfg.CreateSchema {
		return nil
	}

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
		ddl := fmt.Sprintf(ddlTemplate, e.cfg.Table.Metrics, e.cfg.propertiesStr())
		_, err = conn.ExecContext(ctx, ddl)
		if err != nil {
			return err
		}
	}

	return nil
}

func (e *metricsExporter) shutdown(_ context.Context) error {
	if e.client != nil {
		e.client.CloseIdleConnections()
	}
	return nil
}

func (e *metricsExporter) pushMetricData(ctx context.Context, md pmetric.Metrics) error {
	metricMap := initMetricMap(md.DataPointCount())

	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		resourceMetric := md.ResourceMetrics().At(i)
		resource := resourceMetric.Resource()
		resourceAttributes := resource.Attributes()
		serviceName := ""
		v, ok := resourceAttributes.Get(semconv.AttributeServiceName)
		if ok {
			serviceName = v.AsString()
		}

		for j := 0; j < resourceMetric.ScopeMetrics().Len(); j++ {
			scopeMetric := resourceMetric.ScopeMetrics().At(j)

			for k := 0; k < scopeMetric.Metrics().Len(); k++ {
				metric := scopeMetric.Metrics().At(k)

				dm := &dMetric{
					ServiceName:        serviceName,
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
	if metrics.size() <= 0 {
		return nil
	}

	marshal, err := metrics.bytes()
	if err != nil {
		return err
	}

	req, err := streamLoadRequest(ctx, e.cfg, e.cfg.Table.Metrics+metrics.tableSuffix(), marshal)
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

	if !response.success() {
		return fmt.Errorf("failed to push metric data: %s", response.Message)
	}

	return nil
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
