// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package batchperresourceattr // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchperresourceattr"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/multierr"
)

var separator = string([]byte{0x0, 0x1})

// Option configures a batch consumer created by this package.
type Option func(*options)

type options struct {
	injectMetadata bool
}

// WithMetadataInjection enables injecting the batched resource attribute
// values as client.Metadata into the context before forwarding each
// sub-batch to the next consumer. This allows downstream components (e.g.
// an exporterhelper batcher with Partition.MetadataKeys configured) to
// partition requests by the same keys used for batching.
func WithMetadataInjection() Option {
	return func(o *options) {
		o.injectMetadata = true
	}
}

// injectAttrMetadata returns a new context with client.Metadata populated
// from the given attribute map for the specified keys. Keys absent from the
// map are omitted. If no keys are found the original context is returned.
func injectAttrMetadata(ctx context.Context, attrs pcommon.Map, attrKeys []string) context.Context {
	meta := map[string][]string{}
	for _, k := range attrKeys {
		if v, ok := attrs.Get(k); ok {
			meta[k] = []string{v.Str()}
		}
	}
	if len(meta) == 0 {
		return ctx
	}
	return client.NewContext(ctx, client.Info{Metadata: client.NewMetadata(meta)})
}

type batchTraces struct {
	attrKeys       []string
	injectMetadata bool
	next           consumer.Traces
}

func NewBatchPerResourceTraces(attrKey string, next consumer.Traces, opts ...Option) consumer.Traces {
	return NewMultiBatchPerResourceTraces([]string{attrKey}, next, opts...)
}

func NewMultiBatchPerResourceTraces(attrKeys []string, next consumer.Traces, opts ...Option) consumer.Traces {
	o := options{}
	for _, opt := range opts {
		opt(&o)
	}
	return &batchTraces{
		attrKeys:       attrKeys,
		injectMetadata: o.injectMetadata,
		next:           next,
	}
}

// Capabilities returns the capabilities of the next consumer because batchTraces doesn't mutate data itself.
func (bt *batchTraces) Capabilities() consumer.Capabilities {
	return bt.next.Capabilities()
}

func (bt *batchTraces) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	rss := td.ResourceSpans()
	lenRss := rss.Len()
	// If zero or one resource spans just call next.
	if lenRss <= 1 {
		if bt.injectMetadata && lenRss == 1 {
			ctx = injectAttrMetadata(ctx, rss.At(0).Resource().Attributes(), bt.attrKeys)
		}
		return bt.next.ConsumeTraces(ctx, td)
	}

	indicesByAttr := make(map[string][]int)
	for i := range lenRss {
		rs := rss.At(i)
		var attrVal string

		for _, k := range bt.attrKeys {
			if attributeValue, ok := rs.Resource().Attributes().Get(k); ok {
				attrVal = fmt.Sprintf("%s%s%s", attrVal, separator, attributeValue.Str())
			}
		}

		indicesByAttr[attrVal] = append(indicesByAttr[attrVal], i)
	}
	// If there is a single attribute value, then call next.
	if len(indicesByAttr) <= 1 {
		if bt.injectMetadata {
			ctx = injectAttrMetadata(ctx, rss.At(0).Resource().Attributes(), bt.attrKeys)
		}
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
		callCtx := ctx
		if bt.injectMetadata {
			callCtx = injectAttrMetadata(ctx, rss.At(indices[0]).Resource().Attributes(), bt.attrKeys)
		}
		errs = multierr.Append(errs, bt.next.ConsumeTraces(callCtx, tracesForAttr))
	}
	return errs
}

type batchMetrics struct {
	attrKeys       []string
	injectMetadata bool
	next           consumer.Metrics
}

func NewBatchPerResourceMetrics(attrKey string, next consumer.Metrics, opts ...Option) consumer.Metrics {
	return NewMultiBatchPerResourceMetrics([]string{attrKey}, next, opts...)
}

func NewMultiBatchPerResourceMetrics(attrKeys []string, next consumer.Metrics, opts ...Option) consumer.Metrics {
	o := options{}
	for _, opt := range opts {
		opt(&o)
	}
	return &batchMetrics{
		attrKeys:       attrKeys,
		injectMetadata: o.injectMetadata,
		next:           next,
	}
}

// Capabilities returns the capabilities of the next consumer because batchMetrics doesn't mutate data itself.
func (bt *batchMetrics) Capabilities() consumer.Capabilities {
	return bt.next.Capabilities()
}

func (bt *batchMetrics) ConsumeMetrics(ctx context.Context, td pmetric.Metrics) error {
	rms := td.ResourceMetrics()
	lenRms := rms.Len()
	// If zero or one resource metrics just call next.
	if lenRms <= 1 {
		if bt.injectMetadata && lenRms == 1 {
			ctx = injectAttrMetadata(ctx, rms.At(0).Resource().Attributes(), bt.attrKeys)
		}
		return bt.next.ConsumeMetrics(ctx, td)
	}

	indicesByAttr := make(map[string][]int)
	for i := range lenRms {
		rm := rms.At(i)
		var attrVal string
		for _, k := range bt.attrKeys {
			if attributeValue, ok := rm.Resource().Attributes().Get(k); ok {
				attrVal = fmt.Sprintf("%s%s%s", attrVal, separator, attributeValue.Str())
			}
		}
		indicesByAttr[attrVal] = append(indicesByAttr[attrVal], i)
	}
	// If there is a single attribute value, then call next.
	if len(indicesByAttr) <= 1 {
		if bt.injectMetadata {
			ctx = injectAttrMetadata(ctx, rms.At(0).Resource().Attributes(), bt.attrKeys)
		}
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
		callCtx := ctx
		if bt.injectMetadata {
			callCtx = injectAttrMetadata(ctx, rms.At(indices[0]).Resource().Attributes(), bt.attrKeys)
		}
		errs = multierr.Append(errs, bt.next.ConsumeMetrics(callCtx, metricsForAttr))
	}
	return errs
}

type batchLogs struct {
	attrKeys       []string
	injectMetadata bool
	next           consumer.Logs
}

func NewBatchPerResourceLogs(attrKey string, next consumer.Logs, opts ...Option) consumer.Logs {
	return NewMultiBatchPerResourceLogs([]string{attrKey}, next, opts...)
}

func NewMultiBatchPerResourceLogs(attrKeys []string, next consumer.Logs, opts ...Option) consumer.Logs {
	o := options{}
	for _, opt := range opts {
		opt(&o)
	}
	return &batchLogs{
		attrKeys:       attrKeys,
		injectMetadata: o.injectMetadata,
		next:           next,
	}
}

// Capabilities returns the capabilities of the next consumer because batchLogs doesn't mutate data itself.
func (bt *batchLogs) Capabilities() consumer.Capabilities {
	return bt.next.Capabilities()
}

func (bt *batchLogs) ConsumeLogs(ctx context.Context, td plog.Logs) error {
	rls := td.ResourceLogs()
	lenRls := rls.Len()
	// If zero or one resource logs just call next.
	if lenRls <= 1 {
		if bt.injectMetadata && lenRls == 1 {
			ctx = injectAttrMetadata(ctx, rls.At(0).Resource().Attributes(), bt.attrKeys)
		}
		return bt.next.ConsumeLogs(ctx, td)
	}

	indicesByAttr := make(map[string][]int)
	for i := range lenRls {
		rl := rls.At(i)
		var attrVal string
		for _, k := range bt.attrKeys {
			if attributeValue, ok := rl.Resource().Attributes().Get(k); ok {
				attrVal = fmt.Sprintf("%s%s%s", attrVal, separator, attributeValue.Str())
			}
		}
		indicesByAttr[attrVal] = append(indicesByAttr[attrVal], i)
	}
	// If there is a single attribute value, then call next.
	if len(indicesByAttr) <= 1 {
		if bt.injectMetadata {
			ctx = injectAttrMetadata(ctx, rls.At(0).Resource().Attributes(), bt.attrKeys)
		}
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
		callCtx := ctx
		if bt.injectMetadata {
			callCtx = injectAttrMetadata(ctx, rls.At(indices[0]).Resource().Attributes(), bt.attrKeys)
		}
		errs = multierr.Append(errs, bt.next.ConsumeLogs(callCtx, logsForAttr))
	}
	return errs
}
