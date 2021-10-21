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

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/multierr"
)

type batchTraces struct {
	attrKey string
	next    consumer.Traces
}

func NewBatchPerResourceTraces(attrKey string, next consumer.Traces) consumer.Traces {
	return &batchTraces{
		attrKey: attrKey,
		next:    next,
	}
}

// Capabilities implements the consumer interface.
func (bt batchTraces) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
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
		tgt := tracesForAttr.ResourceSpans().AppendEmpty()
		rs.CopyTo(tgt)
	}

	var errs error
	for _, td := range tracesByAttr {
		errs = multierr.Append(errs, bt.next.ConsumeTraces(ctx, td))
	}
	return errs
}

type batchMetrics struct {
	attrKey string
	next    consumer.Metrics
}

func NewBatchPerResourceMetrics(attrKey string, next consumer.Metrics) consumer.Metrics {
	return &batchMetrics{
		attrKey: attrKey,
		next:    next,
	}
}

// Capabilities implements the consumer interface.
func (bt batchMetrics) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
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
		tgt := metricsForAttr.ResourceMetrics().AppendEmpty()
		rm.CopyTo(tgt)
	}

	var errs error
	for _, td := range metricsByAttr {
		errs = multierr.Append(errs, bt.next.ConsumeMetrics(ctx, td))
	}
	return errs
}

type batchLogs struct {
	attrKey string
	next    consumer.Logs
}

func NewBatchPerResourceLogs(attrKey string, next consumer.Logs) consumer.Logs {
	return &batchLogs{
		attrKey: attrKey,
		next:    next,
	}
}

// Capabilities implements the consumer interface.
func (bt batchLogs) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
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
		tgt := logsForAttr.ResourceLogs().AppendEmpty()
		rl.CopyTo(tgt)
	}

	var errs error
	for _, td := range logsByAttr {
		errs = multierr.Append(errs, bt.next.ConsumeLogs(ctx, td))
	}
	return errs
}
