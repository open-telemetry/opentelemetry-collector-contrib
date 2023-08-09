// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package batchperresourceattr // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchperresourceattr"

import (
	"context"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
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
func (bt *batchTraces) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (bt *batchTraces) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	rss := td.ResourceSpans()
	lenRss := rss.Len()
	// If zero or one resource spans just call next.
	if lenRss <= 1 {
		return bt.next.ConsumeTraces(ctx, td)
	}

	indicesByAttr := make(map[string][]int)
	for i := 0; i < lenRss; i++ {
		rs := rss.At(i)
		var attrVal string
		if attributeValue, ok := rs.Resource().Attributes().Get(bt.attrKey); ok {
			attrVal = attributeValue.Str()
		}
		indicesByAttr[attrVal] = append(indicesByAttr[attrVal], i)
	}
	// If there is a single attribute value, then call next.
	if len(indicesByAttr) <= 1 {
		return bt.next.ConsumeTraces(ctx, td)
	}

	// Build the resource spans for each attribute value using CopyTo and call next for each one.
	var errs error
	for _, indices := range indicesByAttr {
		tracesForAttr := ptrace.NewTraces()
		for _, i := range indices {
			rs := rss.At(i)
			rs.CopyTo(tracesForAttr.ResourceSpans().AppendEmpty())
		}
		errs = multierr.Append(errs, bt.next.ConsumeTraces(ctx, tracesForAttr))
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
func (bt *batchMetrics) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (bt *batchMetrics) ConsumeMetrics(ctx context.Context, td pmetric.Metrics) error {
	rms := td.ResourceMetrics()
	lenRms := rms.Len()
	// If zero or one resource metrics just call next.
	if lenRms <= 1 {
		return bt.next.ConsumeMetrics(ctx, td)
	}

	indicesByAttr := make(map[string][]int)
	for i := 0; i < lenRms; i++ {
		rm := rms.At(i)
		var attrVal string
		if attributeValue, ok := rm.Resource().Attributes().Get(bt.attrKey); ok {
			attrVal = attributeValue.Str()
		}
		indicesByAttr[attrVal] = append(indicesByAttr[attrVal], i)
	}
	// If there is a single attribute value, then call next.
	if len(indicesByAttr) <= 1 {
		return bt.next.ConsumeMetrics(ctx, td)
	}

	// Build the resource metrics for each attribute value using CopyTo and call next for each one.
	var errs error
	for _, indices := range indicesByAttr {
		metricsForAttr := pmetric.NewMetrics()
		for _, i := range indices {
			rm := rms.At(i)
			rm.CopyTo(metricsForAttr.ResourceMetrics().AppendEmpty())
		}
		errs = multierr.Append(errs, bt.next.ConsumeMetrics(ctx, metricsForAttr))
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
func (bt *batchLogs) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (bt *batchLogs) ConsumeLogs(ctx context.Context, td plog.Logs) error {
	rls := td.ResourceLogs()
	lenRls := rls.Len()
	// If zero or one resource logs just call next.
	if lenRls <= 1 {
		return bt.next.ConsumeLogs(ctx, td)
	}

	indicesByAttr := make(map[string][]int)
	for i := 0; i < lenRls; i++ {
		rl := rls.At(i)
		var attrVal string
		if attributeValue, ok := rl.Resource().Attributes().Get(bt.attrKey); ok {
			attrVal = attributeValue.Str()
		}
		indicesByAttr[attrVal] = append(indicesByAttr[attrVal], i)
	}
	// If there is a single attribute value, then call next.
	if len(indicesByAttr) <= 1 {
		return bt.next.ConsumeLogs(ctx, td)
	}

	// Build the resource logs for each attribute value using CopyTo and call next for each one.
	var errs error
	for _, indices := range indicesByAttr {
		logsForAttr := plog.NewLogs()
		for _, i := range indices {
			rl := rls.At(i)
			rl.CopyTo(logsForAttr.ResourceLogs().AppendEmpty())
		}
		errs = multierr.Append(errs, bt.next.ConsumeLogs(ctx, logsForAttr))
	}
	return errs
}
