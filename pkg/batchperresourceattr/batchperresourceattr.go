// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package batchperresourceattr

import (
	"context"

	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/pdata"
)

type batchTraces struct {
	attrKey string
	next    consumer.TracesConsumer
}

func NewBatchPerResourceTraces(attrKey string, next consumer.TracesConsumer) consumer.TracesConsumer {
	return &batchTraces{
		attrKey: attrKey,
		next:    next,
	}
}

func (bt *batchTraces) ConsumeTraces(ctx context.Context, td pdata.Traces) error {
	rss := td.ResourceSpans()
	lenRss := rss.Len()
	// If zero or one resource spans just call next.
	if lenRss <= 1 {
		return bt.next.ConsumeTraces(ctx, td)
	}

	tracesByAttr := make(map[string]pdata.Traces)
	for i := 0; i < lenRss; i++ {
		rs := rss.At(i)
		var attrVal string
		if attributeValue, ok := rs.Resource().Attributes().Get(bt.attrKey); ok {
			attrVal = attributeValue.StringVal()
		}

		tracesForAttr, ok := tracesByAttr[attrVal]
		if !ok {
			tracesForAttr = pdata.NewTraces()
			tracesByAttr[attrVal] = tracesForAttr
		}

		// Append ResourceSpan to pdata.Traces for this attribute value.
		tracesForAttr.ResourceSpans().Append(rs)
	}

	var errs []error
	for _, td := range tracesByAttr {
		if err := bt.next.ConsumeTraces(ctx, td); err != nil {
			errs = append(errs, err)
		}
	}
	return componenterror.CombineErrors(errs)
}

type batchMetrics struct {
	attrKey string
	next    consumer.MetricsConsumer
}

func NewBatchPerResourceMetrics(attrKey string, next consumer.MetricsConsumer) consumer.MetricsConsumer {
	return &batchMetrics{
		attrKey: attrKey,
		next:    next,
	}
}

func (bt *batchMetrics) ConsumeMetrics(ctx context.Context, td pdata.Metrics) error {
	rms := td.ResourceMetrics()
	lenRms := rms.Len()
	// If zero or one resource spans just call next.
	if lenRms <= 1 {
		return bt.next.ConsumeMetrics(ctx, td)
	}

	metricsByAttr := make(map[string]pdata.Metrics)
	for i := 0; i < lenRms; i++ {
		rm := rms.At(i)
		var attrVal string
		if attributeValue, ok := rm.Resource().Attributes().Get(bt.attrKey); ok {
			attrVal = attributeValue.StringVal()
		}

		metricsForAttr, ok := metricsByAttr[attrVal]
		if !ok {
			metricsForAttr = pdata.NewMetrics()
			metricsByAttr[attrVal] = metricsForAttr
		}

		// Append ResourceSpan to pdata.Metrics for this attribute value.
		metricsForAttr.ResourceMetrics().Append(rm)
	}

	var errs []error
	for _, td := range metricsByAttr {
		if err := bt.next.ConsumeMetrics(ctx, td); err != nil {
			errs = append(errs, err)
		}
	}
	return componenterror.CombineErrors(errs)
}

type batchLogs struct {
	attrKey string
	next    consumer.LogsConsumer
}

func NewBatchPerResourceLogs(attrKey string, next consumer.LogsConsumer) consumer.LogsConsumer {
	return &batchLogs{
		attrKey: attrKey,
		next:    next,
	}
}

func (bt *batchLogs) ConsumeLogs(ctx context.Context, td pdata.Logs) error {
	rls := td.ResourceLogs()
	lenRls := rls.Len()
	// If zero or one resource spans just call next.
	if lenRls <= 1 {
		return bt.next.ConsumeLogs(ctx, td)
	}

	logsByAttr := make(map[string]pdata.Logs)
	for i := 0; i < lenRls; i++ {
		rl := rls.At(i)
		var attrVal string
		if attributeValue, ok := rl.Resource().Attributes().Get(bt.attrKey); ok {
			attrVal = attributeValue.StringVal()
		}

		logsForAttr, ok := logsByAttr[attrVal]
		if !ok {
			logsForAttr = pdata.NewLogs()
			logsByAttr[attrVal] = logsForAttr
		}

		// Append ResourceSpan to pdata.Logs for this attribute value.
		logsForAttr.ResourceLogs().Append(rl)
	}

	var errs []error
	for _, td := range logsByAttr {
		if err := bt.next.ConsumeLogs(ctx, td); err != nil {
			errs = append(errs, err)
		}
	}
	return componenterror.CombineErrors(errs)
}
