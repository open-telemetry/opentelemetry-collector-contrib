// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package newrelicexporter

import (
	"context"
	"fmt"
	"io"

	"github.com/newrelic/newrelic-telemetry-sdk-go/cumulative"
	"github.com/newrelic/newrelic-telemetry-sdk-go/telemetry"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	name    = "opentelemetry-collector"
	version = "0.0.0"
	product = "NewRelic-Collector-OpenTelemetry"
)

var _ io.Writer = logWriter{}

// logWriter wraps a zap.Logger into an io.Writer.
type logWriter struct {
	logf func(string, ...zapcore.Field)
}

// Write implements io.Writer
func (w logWriter) Write(p []byte) (n int, err error) {
	w.logf(string(p))
	return len(p), nil
}

// exporter exporters OpenTelemetry Collector data to New Relic.
type exporter struct {
	deltaCalculator *cumulative.DeltaCalculator
	harvester       *telemetry.Harvester
}

func newExporter(l *zap.Logger, c configmodels.Exporter) (*exporter, error) {
	nrConfig, ok := c.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config: %#v", c)
	}

	opts := []func(*telemetry.Config){
		nrConfig.HarvestOption,
		telemetry.ConfigBasicErrorLogger(logWriter{l.Error}),
		telemetry.ConfigBasicDebugLogger(logWriter{l.Info}),
		telemetry.ConfigBasicAuditLogger(logWriter{l.Debug}),
	}

	h, err := telemetry.NewHarvester(opts...)
	if nil != err {
		return nil, err
	}

	return &exporter{
		deltaCalculator: cumulative.NewDeltaCalculator(),
		harvester:       h,
	}, nil
}

func (e exporter) pushTraceData(ctx context.Context, td consumerdata.TraceData) (int, error) {
	var errs []error
	goodSpans := 0

	transform := &transformer{
		ServiceName: td.Node.ServiceInfo.Name,
		Resource:    td.Resource,
	}

	for _, span := range td.Spans {
		nrSpan, err := transform.Span(span)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		err = e.harvester.RecordSpan(nrSpan)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		goodSpans++
	}

	e.harvester.HarvestNow(ctx)

	return len(td.Spans) - goodSpans, componenterror.CombineErrors(errs)
}

func (e exporter) pushMetricData(ctx context.Context, md consumerdata.MetricsData) (int, error) {
	var errs []error
	goodMetrics := 0

	transform := &transformer{
		DeltaCalculator: e.deltaCalculator,
		ServiceName:     md.Node.ServiceInfo.Name,
		Resource:        md.Resource,
	}

	for _, metric := range md.Metrics {
		nrMetrics, err := transform.Metric(metric)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		// TODO: optimize this, RecordMetric locks each call.
		for _, m := range nrMetrics {
			e.harvester.RecordMetric(m)
		}
		goodMetrics++
	}

	e.harvester.HarvestNow(ctx)

	return len(md.Metrics) - goodMetrics, componenterror.CombineErrors(errs)
}

func (e exporter) Shutdown(ctx context.Context) error {
	e.harvester.HarvestNow(ctx)
	return nil
}
