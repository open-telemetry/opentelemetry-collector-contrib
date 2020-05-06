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
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"

	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/newrelic/newrelic-telemetry-sdk-go/telemetry"
	"github.com/open-telemetry/opentelemetry-collector/component/componenterror"
	"github.com/open-telemetry/opentelemetry-collector/config/configmodels"
	"github.com/open-telemetry/opentelemetry-collector/consumer/consumerdata"
	"go.opencensus.io/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	name    = "opentelemetry-collector"
	version = "0.1.0"
	product = "NewRelic-Go-OpenTelemetry"
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

type transformer struct {
	ServiceName string
	Resource    *resourcepb.Resource
}

var (
	emptySpan   telemetry.Span
	emptySpanID trace.SpanID
)

func (t *transformer) Span(span *tracepb.Span) (telemetry.Span, error) {
	if span == nil {
		return emptySpan, errors.New("empty span")
	}

	startTime := t.Timestamp(span.StartTime)

	sp := telemetry.Span{
		ID:          hex.Dump(span.SpanId),
		TraceID:     hex.Dump(span.TraceId),
		Name:        span.Name.String(),
		Timestamp:   startTime,
		Duration:    t.Timestamp(span.EndTime).Sub(startTime),
		ServiceName: t.ServiceName,
		Attributes:  t.SpanAttributes(span),
	}

	var parentSpanID trace.SpanID
	copy(parentSpanID[:], span.ParentSpanId)
	if parentSpanID != emptySpanID {
		sp.ParentID = parentSpanID.String()
	}

	return sp, nil
}

func (t *transformer) SpanAttributes(span *tracepb.Span) map[string]interface{} {

	length := 2

	isErr := t.isError(span.Status.Code)
	if isErr {
		length++
	}

	if t.Resource != nil {
		length += len(t.Resource.Labels)
	}

	if span.Attributes != nil {
		length += len(span.Attributes.AttributeMap)
	}

	attrs := make(map[string]interface{}, length)

	// Any existing error attribute will override this.
	if isErr {
		attrs["error"] = true
	}

	if t.Resource != nil {
		for k, v := range t.Resource.Labels {
			attrs[k] = v
		}
	}

	for key, attr := range span.Attributes.AttributeMap {
		if attr == nil || attr.Value == nil {
			continue
		}
		// Default to skipping if unknown type.
		switch v := attr.Value.(type) {
		case *tracepb.AttributeValue_BoolValue:
			attrs[key] = v.BoolValue
		case *tracepb.AttributeValue_IntValue:
			attrs[key] = v.IntValue
		case *tracepb.AttributeValue_StringValue:
			attrs[key] = v.StringValue.String()
		}
	}

	// Default attributes to tell New Relic about this collector.
	// (overrides any existing)
	attrs["collector.name"] = name
	attrs["collector.version"] = version

	return attrs
}

func (t *transformer) Timestamp(ts *timestamp.Timestamp) time.Time {
	if ts == nil {
		return time.Time{}
	}
	return time.Unix(ts.Seconds, int64(ts.Nanos))
}

func (t *transformer) isError(code int32) bool {
	if code == 0 {
		return false
	}
	return true
}

var transformers = sync.Pool{
	New: func() interface{} { return new(transformer) },
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

	opts := append([]func(*telemetry.Config){
		func(cfg *telemetry.Config) {
			cfg.Product = product
			cfg.ProductVersion = version
		},
		telemetry.ConfigBasicErrorLogger(logWriter{l.Error}),
		telemetry.ConfigBasicDebugLogger(logWriter{l.Info}),
		telemetry.ConfigBasicAuditLogger(logWriter{l.Debug}),
		telemetry.ConfigAPIKey(nrConfig.APIKey),
	}, nrConfig.Options...)
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
