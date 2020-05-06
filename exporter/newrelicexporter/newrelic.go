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

	"github.com/newrelic/newrelic-telemetry-sdk-go/telemetry"
	"github.com/open-telemetry/opentelemetry-collector/component/componenterror"
	"github.com/open-telemetry/opentelemetry-collector/config/configmodels"
	"github.com/open-telemetry/opentelemetry-collector/consumer/consumerdata"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	name    = "opentelemetry-collector"
	version = "0.1.0"
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
	harvester *telemetry.Harvester
}

func newExporter(l *zap.Logger, c configmodels.Exporter) (*exporter, error) {
	nrConfig, ok := c.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config: %#v", c)
	}

	opts := append(nrConfig.HarvestOptions(),
		telemetry.ConfigBasicErrorLogger(logWriter{l.Error}),
		telemetry.ConfigBasicDebugLogger(logWriter{l.Info}),
		telemetry.ConfigBasicAuditLogger(logWriter{l.Debug}),
	)

	h, err := telemetry.NewHarvester(opts...)
	if nil != err {
		return nil, err
	}
	return &exporter{h}, nil
}

func (e exporter) pushTraceData(ctx context.Context, td consumerdata.TraceData) (int, error) {
	var errs []error
	goodSpans := 0

	transform := transformers.Get().(*transformer)
	transform.ServiceName = td.Node.ServiceInfo.Name
	transform.Resource = td.Resource

	for _, span := range td.Spans {
		nrSpan, err := transform.Span(span)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		goodSpans++
		e.harvester.RecordSpan(nrSpan)
	}
	transformers.Put(transform)

	return len(td.Spans) - goodSpans, componenterror.CombineErrors(errs)
}

func (e exporter) pushMetricData(ctx context.Context, td consumerdata.MetricsData) (int, error) {
	// FIXME
	return 0, nil
}
