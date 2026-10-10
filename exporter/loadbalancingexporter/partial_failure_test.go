// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter/internal/metadatatest"
)

// partialFailureSignal adapts one signal type to the signal-agnostic partial failure tests.
// Every data item carries its own resource and scope, so an item's identity string also
// covers the resource attributes, scope attributes, and schema URLs it was sent with.
type partialFailureSignal interface {
	name() string
	// build returns a payload with one item per identity.
	build(items []string) any
	// items returns the identity of every item in a payload, sorted.
	items(data any) []string
	marshal(t *testing.T, data any) []byte
	markReadOnly(data any)
	// mutate changes every item of a payload in place.
	mutate(data any)
	typedErr(cause error, data any) error
	// typedData returns the payload of the first matching typed signal error in err's tree.
	typedData(err error) (any, bool)
	// filter returns a copy of the resource entries of data whose item.id is in items.
	filter(data any, items []string) any
	newExporter(logger *zap.Logger, tb *metadata.TelemetryBuilder) partialFailureExporter
	newBackend(endpoint string, consume func(ctx context.Context, data any) error) *wrappedExporter
}

type partialFailureExporter interface {
	exportBatches(ctx context.Context, batches map[*wrappedExporter]any) error
}

func itemIdentity(resourceSchema, item, scopeSchema, scopeName, scopeAttr string) string {
	return fmt.Sprintf("%s|%s|%s|%s|%s", resourceSchema, item, scopeSchema, scopeName, scopeAttr)
}

func sortedItems(items []string) []string {
	sort.Strings(items)
	return items
}

// traces

type tracesSignal struct{}

func (tracesSignal) name() string { return "traces" }

func (tracesSignal) build(items []string) any {
	td := ptrace.NewTraces()
	for _, item := range items {
		rs := td.ResourceSpans().AppendEmpty()
		rs.SetSchemaUrl("https://schema/resource/" + item)
		rs.Resource().Attributes().PutStr("item.id", item)
		ss := rs.ScopeSpans().AppendEmpty()
		ss.SetSchemaUrl("https://schema/scope/" + item)
		ss.Scope().SetName("scope-" + item)
		ss.Scope().Attributes().PutStr("scope.item", item)
		ss.Spans().AppendEmpty().SetName(item)
	}
	return td
}

func (tracesSignal) items(data any) []string {
	td := data.(ptrace.Traces)
	var out []string
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rs := td.ResourceSpans().At(i)
		resItem, _ := rs.Resource().Attributes().Get("item.id")
		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			ss := rs.ScopeSpans().At(j)
			scopeAttr, _ := ss.Scope().Attributes().Get("scope.item")
			for k := 0; k < ss.Spans().Len(); k++ {
				name := ss.Spans().At(k).Name()
				if resItem.Str() != name {
					name += "@" + resItem.Str()
				}
				out = append(out, itemIdentity(rs.SchemaUrl(), name, ss.SchemaUrl(), ss.Scope().Name(), scopeAttr.Str()))
			}
		}
	}
	return sortedItems(out)
}

func (tracesSignal) marshal(t *testing.T, data any) []byte {
	b, err := (&ptrace.ProtoMarshaler{}).MarshalTraces(data.(ptrace.Traces))
	require.NoError(t, err)
	return b
}

func (tracesSignal) markReadOnly(data any) { data.(ptrace.Traces).MarkReadOnly() }

func (tracesSignal) mutate(data any) {
	rss := data.(ptrace.Traces).ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		rs.Resource().Attributes().PutStr("mutated", "yes")
		rs.ScopeSpans().At(0).Spans().At(0).SetName("mutated")
	}
}

func (tracesSignal) typedErr(cause error, data any) error {
	return consumererror.NewTraces(cause, data.(ptrace.Traces))
}

func (tracesSignal) typedData(err error) (any, bool) {
	tracesErr, ok := errors.AsType[consumererror.Traces](err)
	if !ok {
		return nil, false
	}
	return tracesErr.Data(), true
}

type tracesPartialFailureExporter struct{ e *traceExporterImp }

func (x tracesPartialFailureExporter) exportBatches(ctx context.Context, batches map[*wrappedExporter]any) error {
	typed := make(exporterTraces, len(batches))
	for exp, data := range batches {
		typed[exp] = data.(ptrace.Traces)
	}
	return x.e.exportBatches(ctx, typed)
}

func (tracesSignal) newExporter(logger *zap.Logger, tb *metadata.TelemetryBuilder) partialFailureExporter {
	return tracesPartialFailureExporter{e: &traceExporterImp{logger: logger, telemetry: tb}}
}

func (tracesSignal) filter(data any, items []string) any {
	out := ptrace.NewTraces()
	src := data.(ptrace.Traces).ResourceSpans()
	for i := 0; i < src.Len(); i++ {
		id, _ := src.At(i).Resource().Attributes().Get("item.id")
		if slices.Contains(items, id.Str()) {
			src.At(i).CopyTo(out.ResourceSpans().AppendEmpty())
		}
	}
	return out
}

func (tracesSignal) newBackend(endpoint string, consume func(ctx context.Context, data any) error) *wrappedExporter {
	return newWrappedExporter(newMockTracesExporter(func(ctx context.Context, td ptrace.Traces) error {
		return consume(ctx, td)
	}), endpoint)
}

// logs

type logsSignal struct{}

func (logsSignal) name() string { return "logs" }

func (logsSignal) build(items []string) any {
	ld := plog.NewLogs()
	for _, item := range items {
		rl := ld.ResourceLogs().AppendEmpty()
		rl.SetSchemaUrl("https://schema/resource/" + item)
		rl.Resource().Attributes().PutStr("item.id", item)
		sl := rl.ScopeLogs().AppendEmpty()
		sl.SetSchemaUrl("https://schema/scope/" + item)
		sl.Scope().SetName("scope-" + item)
		sl.Scope().Attributes().PutStr("scope.item", item)
		sl.LogRecords().AppendEmpty().Body().SetStr(item)
	}
	return ld
}

func (logsSignal) items(data any) []string {
	ld := data.(plog.Logs)
	var out []string
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rl := ld.ResourceLogs().At(i)
		resItem, _ := rl.Resource().Attributes().Get("item.id")
		for j := 0; j < rl.ScopeLogs().Len(); j++ {
			sl := rl.ScopeLogs().At(j)
			scopeAttr, _ := sl.Scope().Attributes().Get("scope.item")
			for k := 0; k < sl.LogRecords().Len(); k++ {
				body := sl.LogRecords().At(k).Body().Str()
				if resItem.Str() != body {
					body += "@" + resItem.Str()
				}
				out = append(out, itemIdentity(rl.SchemaUrl(), body, sl.SchemaUrl(), sl.Scope().Name(), scopeAttr.Str()))
			}
		}
	}
	return sortedItems(out)
}

func (logsSignal) marshal(t *testing.T, data any) []byte {
	b, err := (&plog.ProtoMarshaler{}).MarshalLogs(data.(plog.Logs))
	require.NoError(t, err)
	return b
}

func (logsSignal) markReadOnly(data any) { data.(plog.Logs).MarkReadOnly() }

func (logsSignal) mutate(data any) {
	rls := data.(plog.Logs).ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		rl := rls.At(i)
		rl.Resource().Attributes().PutStr("mutated", "yes")
		rl.ScopeLogs().At(0).LogRecords().At(0).Body().SetStr("mutated")
	}
}

func (logsSignal) typedErr(cause error, data any) error {
	return consumererror.NewLogs(cause, data.(plog.Logs))
}

func (logsSignal) typedData(err error) (any, bool) {
	logsErr, ok := errors.AsType[consumererror.Logs](err)
	if !ok {
		return nil, false
	}
	return logsErr.Data(), true
}

type logsPartialFailureExporter struct{ e *logExporterImp }

func (x logsPartialFailureExporter) exportBatches(ctx context.Context, batches map[*wrappedExporter]any) error {
	typed := make(exporterLogs, len(batches))
	for exp, data := range batches {
		typed[exp] = data.(plog.Logs)
	}
	return x.e.exportBatches(ctx, typed)
}

func (logsSignal) newExporter(logger *zap.Logger, tb *metadata.TelemetryBuilder) partialFailureExporter {
	return logsPartialFailureExporter{e: &logExporterImp{logger: logger, telemetry: tb}}
}

func (logsSignal) filter(data any, items []string) any {
	out := plog.NewLogs()
	src := data.(plog.Logs).ResourceLogs()
	for i := 0; i < src.Len(); i++ {
		id, _ := src.At(i).Resource().Attributes().Get("item.id")
		if slices.Contains(items, id.Str()) {
			src.At(i).CopyTo(out.ResourceLogs().AppendEmpty())
		}
	}
	return out
}

func (logsSignal) newBackend(endpoint string, consume func(ctx context.Context, data any) error) *wrappedExporter {
	return newWrappedExporter(newMockLogsExporter(func(ctx context.Context, ld plog.Logs) error {
		return consume(ctx, ld)
	}), endpoint)
}

// metrics

type metricsSignal struct{}

func (metricsSignal) name() string { return "metrics" }

func (metricsSignal) build(items []string) any {
	md := pmetric.NewMetrics()
	for _, item := range items {
		rm := md.ResourceMetrics().AppendEmpty()
		rm.SetSchemaUrl("https://schema/resource/" + item)
		rm.Resource().Attributes().PutStr("item.id", item)
		sm := rm.ScopeMetrics().AppendEmpty()
		sm.SetSchemaUrl("https://schema/scope/" + item)
		sm.Scope().SetName("scope-" + item)
		sm.Scope().Attributes().PutStr("scope.item", item)
		m := sm.Metrics().AppendEmpty()
		m.SetName(item)
		m.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(1)
	}
	return md
}

func (metricsSignal) items(data any) []string {
	md := data.(pmetric.Metrics)
	var out []string
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		resItem, _ := rm.Resource().Attributes().Get("item.id")
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			scopeAttr, _ := sm.Scope().Attributes().Get("scope.item")
			for k := 0; k < sm.Metrics().Len(); k++ {
				m := sm.Metrics().At(k)
				name := m.Name()
				if resItem.Str() != name {
					name += "@" + resItem.Str()
				}
				// One identity per data point, so duplicated data points are detected.
				for range m.Gauge().DataPoints().Len() {
					out = append(out, itemIdentity(rm.SchemaUrl(), name, sm.SchemaUrl(), sm.Scope().Name(), scopeAttr.Str()))
				}
			}
		}
	}
	return sortedItems(out)
}

func (metricsSignal) marshal(t *testing.T, data any) []byte {
	b, err := (&pmetric.ProtoMarshaler{}).MarshalMetrics(data.(pmetric.Metrics))
	require.NoError(t, err)
	return b
}

func (metricsSignal) markReadOnly(data any) { data.(pmetric.Metrics).MarkReadOnly() }

func (metricsSignal) mutate(data any) {
	rms := data.(pmetric.Metrics).ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		rm.Resource().Attributes().PutStr("mutated", "yes")
		rm.ScopeMetrics().At(0).Metrics().At(0).SetName("mutated")
	}
}

func (metricsSignal) typedErr(cause error, data any) error {
	return consumererror.NewMetrics(cause, data.(pmetric.Metrics))
}

func (metricsSignal) typedData(err error) (any, bool) {
	metricsErr, ok := errors.AsType[consumererror.Metrics](err)
	if !ok {
		return nil, false
	}
	return metricsErr.Data(), true
}

type metricsPartialFailureExporter struct{ e *metricExporterImp }

func (x metricsPartialFailureExporter) exportBatches(ctx context.Context, batches map[*wrappedExporter]any) error {
	typed := make(exporterMetrics, len(batches))
	for exp, data := range batches {
		typed[exp] = data.(pmetric.Metrics)
	}
	return x.e.exportBatches(ctx, typed)
}

func (metricsSignal) newExporter(logger *zap.Logger, tb *metadata.TelemetryBuilder) partialFailureExporter {
	return metricsPartialFailureExporter{e: &metricExporterImp{logger: logger, telemetry: tb}}
}

func (metricsSignal) filter(data any, items []string) any {
	out := pmetric.NewMetrics()
	src := data.(pmetric.Metrics).ResourceMetrics()
	for i := 0; i < src.Len(); i++ {
		id, _ := src.At(i).Resource().Attributes().Get("item.id")
		if slices.Contains(items, id.Str()) {
			src.At(i).CopyTo(out.ResourceMetrics().AppendEmpty())
		}
	}
	return out
}

func (metricsSignal) newBackend(endpoint string, consume func(ctx context.Context, data any) error) *wrappedExporter {
	return newWrappedExporter(newMockMetricsExporter(func(ctx context.Context, md pmetric.Metrics) error {
		return consume(ctx, md)
	}), endpoint)
}

var partialFailureSignals = []partialFailureSignal{tracesSignal{}, logsSignal{}, metricsSignal{}}

// backendSpec describes one backend in an exportBatches call.
type backendSpec struct {
	items []string
	fail  bool
	// reported, when non-nil, makes the backend return a typed partial failure carrying only
	// these items. An empty non-nil slice reports an empty subset.
	reported []string
	// wrapTyped wraps the typed partial failure with fmt.Errorf("%w").
	wrapTyped bool
	permanent bool
	// permanentInsideTyped builds typed(NewPermanent(cause)) rather than NewPermanent(typed(cause)).
	permanentInsideTyped bool
}

type backendRun struct {
	cause    error
	err      error
	exp      *wrappedExporter
	calls    int
	input    any
	reported any
}

// runExportBatches calls exportBatches once with one backend per spec. Each backend returns
// the error built from its spec on every call.
func runExportBatches(t *testing.T, sig partialFailureSignal, logger *zap.Logger, tb *metadata.TelemetryBuilder, specs []backendSpec, readOnly bool) ([]*backendRun, error) {
	if tb == nil {
		_, tb = getTelemetryAssets(t)
	}
	runs := make([]*backendRun, len(specs))
	batches := make(map[*wrappedExporter]any, len(specs))
	for i, spec := range specs {
		run := &backendRun{cause: fmt.Errorf("backend %d failed", i), input: sig.build(spec.items)}
		if spec.fail {
			err := run.cause
			if spec.permanentInsideTyped {
				err = consumererror.NewPermanent(err)
			}
			if spec.reported != nil {
				run.reported = sig.build(spec.reported)
				err = sig.typedErr(err, run.reported)
				if spec.wrapTyped {
					err = fmt.Errorf("sub-exporter: %w", err)
				}
			}
			if spec.permanent && !spec.permanentInsideTyped {
				err = consumererror.NewPermanent(err)
			}
			run.err = err
		}
		if readOnly {
			sig.markReadOnly(run.input)
			if run.reported != nil {
				sig.markReadOnly(run.reported)
			}
		}
		run.exp = sig.newBackend(endpointWithPort(fmt.Sprintf("backend-%d", i)), func(context.Context, any) error {
			run.calls++
			return run.err
		})
		run.exp.consumeWG.Add(1)
		batches[run.exp] = run.input
		runs[i] = run
	}
	err := sig.newExporter(logger, tb).exportBatches(t.Context(), batches)
	for i, run := range runs {
		assert.Equal(t, 1, run.calls, "backend %d must be attempted exactly once", i)
		assert.True(t, waitGroupReturns(&run.exp.consumeWG), "backend %d consumeWG not released", i)
	}
	return runs, err
}

func expectedItems(sig partialFailureSignal, items ...[]string) []string {
	var all []string
	for _, i := range items {
		all = append(all, i...)
	}
	return sig.items(sig.build(all))
}

func TestExportBatchesPartialFailureClassification(t *testing.T) {
	tests := []struct {
		name  string
		specs []backendSpec
		// wantNil expects a nil result.
		wantNil       bool
		wantPermanent bool
		wantItems     []string
		// reachable and unreachable are indexes of backend causes.
		reachable   []int
		unreachable []int
		// wantDropWarnings is the number of warnings for permanently dropped data.
		wantDropWarnings int
	}{
		{
			name:    "all backends succeed",
			specs:   []backendSpec{{items: []string{"a"}}, {items: []string{"b"}}},
			wantNil: true,
		},
		{
			name:      "one plain retryable error",
			specs:     []backendSpec{{items: []string{"a"}}, {items: []string{"b1", "b2"}, fail: true}},
			wantItems: []string{"b1", "b2"},
			reachable: []int{1},
		},
		{
			name:      "one typed partial error",
			specs:     []backendSpec{{items: []string{"a"}}, {items: []string{"b1", "b2"}, fail: true, reported: []string{"b2"}}},
			wantItems: []string{"b2"},
			reachable: []int{1},
		},
		{
			name:      "typed partial error wrapped with %w",
			specs:     []backendSpec{{items: []string{"a"}}, {items: []string{"b1", "b2"}, fail: true, reported: []string{"b1"}, wrapTyped: true}},
			wantItems: []string{"b1"},
			reachable: []int{1},
		},
		{
			name: "two retryable failed backends",
			specs: []backendSpec{
				{items: []string{"a"}},
				{items: []string{"b1", "b2"}, fail: true, reported: []string{"b2"}},
				{items: []string{"c1", "c2"}, fail: true},
			},
			wantItems: []string{"b2", "c1", "c2"},
			reachable: []int{1, 2},
		},
		{
			name: "permanent plus retryable",
			specs: []backendSpec{
				{items: []string{"a"}},
				{items: []string{"p1", "p2"}, fail: true, permanent: true},
				{items: []string{"r1", "r2"}, fail: true, reported: []string{"r2"}},
			},
			wantItems:        []string{"r2"},
			reachable:        []int{2},
			unreachable:      []int{1},
			wantDropWarnings: 1,
		},
		{
			name: "typed permanent wrapping plus retryable",
			specs: []backendSpec{
				{items: []string{"a"}},
				{items: []string{"p1", "p2"}, fail: true, permanent: true, reported: []string{"p1"}},
				{items: []string{"q1"}, fail: true, permanent: true, permanentInsideTyped: true, reported: []string{"q1"}, wrapTyped: true},
				{items: []string{"r1"}, fail: true},
			},
			wantItems:        []string{"r1"},
			reachable:        []int{3},
			unreachable:      []int{1, 2},
			wantDropWarnings: 2,
		},
		{
			name: "only permanent failures",
			specs: []backendSpec{
				{items: []string{"a"}},
				{items: []string{"p1", "p2"}, fail: true, permanent: true, reported: []string{"p2"}},
				{items: []string{"q1"}, fail: true, permanent: true, permanentInsideTyped: true, reported: []string{"q1"}},
				{items: []string{"s1"}, fail: true, permanent: true},
			},
			wantPermanent: true,
			wantItems:     []string{"p2", "q1", "s1"},
			reachable:     []int{1, 2, 3},
		},
		{
			name: "empty typed failed subset",
			specs: []backendSpec{
				{items: []string{"a"}},
				{items: []string{"b1"}, fail: true, reported: []string{}},
			},
			wantItems: []string{},
			reachable: []int{1},
		},
	}
	for _, sig := range partialFailureSignals {
		for _, tt := range tests {
			t.Run(sig.name()+"/"+tt.name, func(t *testing.T) {
				core, logs := observer.New(zapcore.WarnLevel)
				runs, err := runExportBatches(t, sig, zap.New(core), nil, tt.specs, false)
				if tt.wantNil {
					require.NoError(t, err)
					assert.Zero(t, logs.Len())
					return
				}
				require.Error(t, err)
				assert.Equal(t, tt.wantPermanent, consumererror.IsPermanent(err), "IsPermanent: %v", err)
				data, ok := sig.typedData(err)
				require.True(t, ok, "expected a typed %s error, got %T: %v", sig.name(), err, err)
				assert.Equal(t, expectedItems(sig, tt.wantItems), sig.items(data))
				for _, i := range tt.reachable {
					assert.ErrorIs(t, err, runs[i].cause, "cause of backend %d must be reachable", i)
				}
				for _, i := range tt.unreachable {
					assert.NotErrorIs(t, err, runs[i].cause, "cause of backend %d must not be reachable", i)
				}
				assert.Equal(t, tt.wantDropWarnings, logs.FilterMessageSnippet("permanently").Len(), "drop warnings: %v", logs.All())
			})
		}
	}
}

// TestExportBatchesPreservesFailurePayloads checks that aggregation copies the failed data:
// the routed input batches and every payload carried by a backend error are unchanged after
// aggregation, and stay unchanged when the returned aggregate is modified.
func TestExportBatchesPreservesFailurePayloads(t *testing.T) {
	specs := map[string][]backendSpec{
		"retryable": {
			{items: []string{"a"}},
			{items: []string{"b1", "b2"}, fail: true, reported: []string{"b2"}},
			{items: []string{"c1", "c2"}, fail: true, reported: []string{"c1"}},
			{items: []string{"d1"}, fail: true},
		},
		"permanent": {
			{items: []string{"b1", "b2"}, fail: true, permanent: true, reported: []string{"b2"}},
			{items: []string{"c1", "c2"}, fail: true, permanent: true, reported: []string{"c1"}},
			{items: []string{"d1"}, fail: true, permanent: true},
		},
	}
	for _, sig := range partialFailureSignals {
		for name, specs := range specs {
			for _, readOnly := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/readOnly=%v", sig.name(), name, readOnly), func(t *testing.T) {
					var err error
					var runs []*backendRun
					require.NotPanics(t, func() {
						runs, err = runExportBatches(t, sig, zap.NewNop(), nil, specs, readOnly)
					})
					inputs := make([][]byte, len(runs))
					reported := make([][]byte, len(runs))
					for i := range runs {
						inputs[i] = sig.marshal(t, sig.build(specs[i].items))
						if specs[i].reported != nil {
							reported[i] = sig.marshal(t, sig.build(specs[i].reported))
						}
					}
					check := func(stage string) {
						for i, run := range runs {
							assert.Equal(t, inputs[i], sig.marshal(t, run.input), "%s: input of backend %d changed", stage, i)
							if run.reported != nil {
								assert.Equal(t, reported[i], sig.marshal(t, run.reported), "%s: error payload of backend %d changed", stage, i)
							}
						}
					}
					check("after aggregation")

					data, ok := sig.typedData(err)
					require.True(t, ok)
					assert.Equal(t, expectedItems(sig, []string{"b2", "c1", "d1"}), sig.items(data))
					sig.mutate(data)
					check("after modifying the aggregate")
				})
			}
		}
	}
}

// TestExportMetricsBatchesMergesFailureStructure checks that failed metric payloads that share
// a resource and scope are merged into one structure, keeping every data point and metric kind.
func TestExportMetricsBatchesMergesFailureStructure(t *testing.T) {
	build := func(dpValue int64) pmetric.Metrics {
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()
		rm.SetSchemaUrl("https://schema/resource")
		rm.Resource().Attributes().PutStr("service.name", "svc")
		sm := rm.ScopeMetrics().AppendEmpty()
		sm.SetSchemaUrl("https://schema/scope")
		sm.Scope().SetName("scope")
		sm.Scope().Attributes().PutStr("scope.attr", "v")

		gauge := sm.Metrics().AppendEmpty()
		gauge.SetName("gauge")
		gauge.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(dpValue)

		sum := sm.Metrics().AppendEmpty()
		sum.SetName("sum")
		sum.SetEmptySum().SetIsMonotonic(true)
		sum.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		sum.Sum().DataPoints().AppendEmpty().SetIntValue(dpValue)

		hist := sm.Metrics().AppendEmpty()
		hist.SetName("histogram")
		hist.SetEmptyHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		hist.Histogram().DataPoints().AppendEmpty().SetCount(uint64(dpValue))

		exp := sm.Metrics().AppendEmpty()
		exp.SetName("exponential_histogram")
		exp.SetEmptyExponentialHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		exp.ExponentialHistogram().DataPoints().AppendEmpty().SetCount(uint64(dpValue))

		summary := sm.Metrics().AppendEmpty()
		summary.SetName("summary")
		summary.SetEmptySummary().DataPoints().AppendEmpty().SetCount(uint64(dpValue))
		return md
	}

	_, tb := getTelemetryAssets(t)
	e := &metricExporterImp{logger: zap.NewNop(), telemetry: tb}
	batches := exporterMetrics{}
	for i := range 2 {
		md := build(int64(i + 1))
		exp := newWrappedExporter(newMockMetricsExporter(func(context.Context, pmetric.Metrics) error {
			return fmt.Errorf("backend %d failed", i)
		}), endpointWithPort(fmt.Sprintf("backend-%d", i)))
		exp.consumeWG.Add(1)
		batches[exp] = md
	}

	err := e.exportBatches(t.Context(), batches)
	metricsErr, ok := errors.AsType[consumererror.Metrics](err)
	require.True(t, ok)
	got := metricsErr.Data()

	require.Equal(t, 1, got.ResourceMetrics().Len())
	rm := got.ResourceMetrics().At(0)
	assert.Equal(t, "https://schema/resource", rm.SchemaUrl())
	assert.Equal(t, map[string]any{"service.name": "svc"}, rm.Resource().Attributes().AsRaw())
	require.Equal(t, 1, rm.ScopeMetrics().Len())
	sm := rm.ScopeMetrics().At(0)
	assert.Equal(t, "https://schema/scope", sm.SchemaUrl())
	assert.Equal(t, "scope", sm.Scope().Name())
	assert.Equal(t, map[string]any{"scope.attr": "v"}, sm.Scope().Attributes().AsRaw())
	require.Equal(t, 5, sm.Metrics().Len())
	assert.Equal(t, 10, got.DataPointCount(), "every data point of every metric kind must be kept")

	values := map[int64]bool{}
	for i := 0; i < sm.Metrics().Len(); i++ {
		m := sm.Metrics().At(i)
		switch m.Type() {
		case pmetric.MetricTypeGauge:
			for j := 0; j < m.Gauge().DataPoints().Len(); j++ {
				values[m.Gauge().DataPoints().At(j).IntValue()] = true
			}
		case pmetric.MetricTypeSum:
			assert.True(t, m.Sum().IsMonotonic())
			assert.Equal(t, pmetric.AggregationTemporalityCumulative, m.Sum().AggregationTemporality())
		case pmetric.MetricTypeHistogram:
			assert.Equal(t, pmetric.AggregationTemporalityDelta, m.Histogram().AggregationTemporality())
		case pmetric.MetricTypeExponentialHistogram:
			assert.Equal(t, pmetric.AggregationTemporalityDelta, m.ExponentialHistogram().AggregationTemporality())
		case pmetric.MetricTypeSummary:
		default:
			t.Errorf("unexpected metric type %v", m.Type())
		}
	}
	assert.Equal(t, map[int64]bool{1: true, 2: true}, values)
}

// abortingResolver resolves the first two identifiers to exp and fails on the third, so a
// reservation always exists before the error regardless of map iteration order.
func abortingResolver(exp *wrappedExporter) func([]byte) (*wrappedExporter, string, error) {
	calls := 0
	return func([]byte) (*wrappedExporter, string, error) {
		calls++
		if calls >= 3 {
			return nil, "", errors.New("couldn't find the exporter")
		}
		return exp, "endpoint", nil
	}
}

func TestGroupLogsByExporterReleasesReservationsOnResolveError(t *testing.T) {
	consumed := false
	exp := newWrappedExporter(newMockLogsExporter(func(context.Context, plog.Logs) error {
		consumed = true
		return nil
	}), endpointWithPort("endpoint"))
	batches := map[string]plog.Logs{
		"a": simpleLogWithServiceName("a"),
		"b": simpleLogWithServiceName("b"),
		"c": simpleLogWithServiceName("c"),
	}

	_, err := groupLogsByExporter(batches, abortingResolver(exp))
	require.Error(t, err)
	assert.False(t, consumed, "no backend may receive data after a resolution error")
	// A second Done() would panic with a negative counter; a missing one blocks Wait().
	require.True(t, waitGroupReturns(&exp.consumeWG), "consumeWG leaked on resolve error")
	require.NoError(t, exp.Shutdown(t.Context()))
}

func TestGroupLogsByExporterReservesOncePerExporter(t *testing.T) {
	exp := newWrappedExporter(newNopMockLogsExporter(), endpointWithPort("endpoint"))
	batches := map[string]plog.Logs{
		"a": simpleLogWithServiceName("a"),
		"b": simpleLogWithServiceName("b"),
		"c": simpleLogWithServiceName("c"),
	}

	grouped, err := groupLogsByExporter(batches, func([]byte) (*wrappedExporter, string, error) {
		return exp, "endpoint", nil
	})
	require.NoError(t, err)
	require.Len(t, grouped, 1)
	assert.Equal(t, 3, grouped[exp].LogRecordCount())
	exp.consumeWG.Done()
	require.True(t, waitGroupReturns(&exp.consumeWG), "more than one reservation for one exporter")
}

func TestGroupMetricsByExporterReleasesReservationsOnResolveError(t *testing.T) {
	consumed := false
	exp := newWrappedExporter(newMockMetricsExporter(func(context.Context, pmetric.Metrics) error {
		consumed = true
		return nil
	}), endpointWithPort("endpoint"))
	batches := map[string]pmetric.Metrics{}
	for _, svc := range []string{"a", "b", "c"} {
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()
		rm.Resource().Attributes().PutStr("service.name", svc)
		appendSimpleMetricWithID(rm, svc)
		batches[svc] = md
	}

	_, err := groupMetricsByExporter(batches, abortingResolver(exp))
	require.Error(t, err)
	assert.False(t, consumed, "no backend may receive data after a resolution error")
	require.True(t, waitGroupReturns(&exp.consumeWG), "consumeWG leaked on resolve error")
	require.NoError(t, exp.Shutdown(t.Context()))
}

func TestGroupMetricsByExporterReservesOncePerExporter(t *testing.T) {
	exp := newWrappedExporter(newNopMockMetricsExporter(), endpointWithPort("endpoint"))
	batches := map[string]pmetric.Metrics{}
	for _, svc := range []string{"a", "b", "c"} {
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()
		rm.Resource().Attributes().PutStr("service.name", svc)
		appendSimpleMetricWithID(rm, svc)
		batches[svc] = md
	}

	grouped, err := groupMetricsByExporter(batches, func([]byte) (*wrappedExporter, string, error) {
		return exp, "endpoint", nil
	})
	require.NoError(t, err)
	require.Len(t, grouped, 1)
	assert.Equal(t, 3, grouped[exp].ResourceMetrics().Len())
	exp.consumeWG.Done()
	require.True(t, waitGroupReturns(&exp.consumeWG), "more than one reservation for one exporter")
}

// recordingBackend records a copy of every payload it receives and answers each call with
// respond, which gets the zero-based call number.
type recordingBackend struct {
	mu       sync.Mutex
	received [][]string
	respond  func(call int, data any) error
}

func (b *recordingBackend) consume(sig partialFailureSignal) func(context.Context, any) error {
	return func(_ context.Context, data any) error {
		b.mu.Lock()
		call := len(b.received)
		b.received = append(b.received, sig.items(data))
		b.mu.Unlock()
		if b.respond == nil {
			return nil
		}
		return b.respond(call, data)
	}
}

func (b *recordingBackend) calls() [][]string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.received
}

func testRetryConfig(maxElapsed time.Duration) configretry.BackOffConfig {
	return configretry.BackOffConfig{
		Enabled:             true,
		InitialInterval:     time.Millisecond,
		RandomizationFactor: 0,
		Multiplier:          1,
		MaxInterval:         time.Millisecond,
		MaxElapsedTime:      maxElapsed,
	}
}

// newRetryHarness builds the signal's exporter over one recording backend per entry, and
// wraps its ConsumeX with a real exporterhelper configured with retry and without a queue, so
// every retry is driven by the helper's own retry sender. The returned function sends one
// item per map entry, routed to the backend at the given index (by trace ID for traces, by
// service name for logs and metrics).
func newRetryHarness(t *testing.T, sig partialFailureSignal, backends []*recordingBackend, retry configretry.BackOffConfig) func(ctx context.Context, items map[string]int) error {
	ts, tb := getTelemetryAssets(t)
	endpoints := make([]string, len(backends))
	exporters := make(map[string]*wrappedExporter, len(backends))
	for i, b := range backends {
		endpoints[i] = fmt.Sprintf("endpoint-%d", i+1)
		exporters[endpointWithPort(endpoints[i])] = sig.newBackend(endpointWithPort(endpoints[i]), b.consume(sig))
	}
	lb := &loadBalancer{ring: newHashRing(endpoints), exporters: exporters}
	t.Cleanup(func() {
		for endpoint, exp := range exporters {
			assert.True(t, waitGroupReturns(&exp.consumeWG), "consumeWG of %s not released", endpoint)
		}
	})

	opts := []exporterhelper.Option{
		exporterhelper.WithRetry(retry),
		exporterhelper.WithQueue(configoptional.None[exporterhelper.QueueBatchConfig]()),
	}
	var helper component.Component
	var consume func(ctx context.Context, items map[string]int) error
	switch sig.(type) {
	case tracesSignal:
		e := &traceExporterImp{loadBalancer: lb, routingKey: traceIDRouting, logger: ts.Logger, telemetry: tb}
		h, err := exporterhelper.NewTraces(t.Context(), ts, &Config{}, e.ConsumeTraces, opts...)
		require.NoError(t, err)
		helper = h
		consume = func(ctx context.Context, items map[string]int) error {
			td := ptrace.NewTraces()
			for item, idx := range items {
				part := sig.build([]string{item}).(ptrace.Traces)
				part.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).SetTraceID(traceIDForEndpoint(t, lb.ring, endpoints[idx]))
				part.ResourceSpans().MoveAndAppendTo(td.ResourceSpans())
			}
			return h.ConsumeTraces(ctx, td)
		}
	case logsSignal:
		e := &logExporterImp{loadBalancer: lb, routingKey: svcRouting, logger: ts.Logger, telemetry: tb}
		h, err := exporterhelper.NewLogs(t.Context(), ts, &Config{}, e.ConsumeLogs, opts...)
		require.NoError(t, err)
		helper = h
		consume = func(ctx context.Context, items map[string]int) error {
			ld := plog.NewLogs()
			for item, idx := range items {
				part := sig.build([]string{item}).(plog.Logs)
				part.ResourceLogs().At(0).Resource().Attributes().PutStr("service.name", serviceNameForEndpoint(t, lb.ring, endpoints[idx]))
				part.ResourceLogs().MoveAndAppendTo(ld.ResourceLogs())
			}
			return h.ConsumeLogs(ctx, ld)
		}
	case metricsSignal:
		e := &metricExporterImp{loadBalancer: lb, routingKey: svcRouting, logger: ts.Logger, telemetry: tb}
		h, err := exporterhelper.NewMetrics(t.Context(), ts, &Config{}, e.ConsumeMetrics, opts...)
		require.NoError(t, err)
		helper = h
		consume = func(ctx context.Context, items map[string]int) error {
			md := pmetric.NewMetrics()
			for item, idx := range items {
				part := sig.build([]string{item}).(pmetric.Metrics)
				part.ResourceMetrics().At(0).Resource().Attributes().PutStr("service.name", serviceNameForEndpoint(t, lb.ring, endpoints[idx]))
				part.ResourceMetrics().MoveAndAppendTo(md.ResourceMetrics())
			}
			return h.ConsumeMetrics(ctx, md)
		}
	}
	require.NoError(t, helper.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { assert.NoError(t, helper.Shutdown(context.WithoutCancel(t.Context()))) })
	return consume
}

func TestRetryResendsOnlyFailedData(t *testing.T) {
	errBackend := errors.New("backend unavailable")
	for _, sig := range partialFailureSignals {
		t.Run(sig.name()+"/temporary failure", func(t *testing.T) {
			healthy := &recordingBackend{}
			flaky := &recordingBackend{respond: func(call int, _ any) error {
				if call == 0 {
					return errBackend
				}
				return nil
			}}
			consume := newRetryHarness(t, sig, []*recordingBackend{healthy, flaky}, testRetryConfig(5*time.Second))

			require.NoError(t, consume(t.Context(), map[string]int{"h1": 0, "f1": 1, "f2": 1}))
			assert.Equal(t, [][]string{expectedItems(sig, []string{"h1"})}, healthy.calls())
			assert.Equal(t, [][]string{expectedItems(sig, []string{"f1", "f2"}), expectedItems(sig, []string{"f1", "f2"})}, flaky.calls())
		})

		t.Run(sig.name()+"/typed partial failure", func(t *testing.T) {
			healthy := &recordingBackend{}
			partial := &recordingBackend{respond: func(call int, data any) error {
				if call == 0 {
					// accept f1, report f2 as failed
					return sig.typedErr(errBackend, sig.filter(data, []string{"f2"}))
				}
				return nil
			}}
			consume := newRetryHarness(t, sig, []*recordingBackend{healthy, partial}, testRetryConfig(5*time.Second))

			require.NoError(t, consume(t.Context(), map[string]int{"h1": 0, "f1": 1, "f2": 1}))
			assert.Equal(t, [][]string{expectedItems(sig, []string{"h1"})}, healthy.calls())
			assert.Equal(t, [][]string{expectedItems(sig, []string{"f1", "f2"}), expectedItems(sig, []string{"f2"})}, partial.calls())
		})

		t.Run(sig.name()+"/mixed permanent and retryable failures", func(t *testing.T) {
			healthy := &recordingBackend{}
			rejecting := &recordingBackend{respond: func(int, any) error {
				return consumererror.NewPermanent(errors.New("rejected"))
			}}
			flaky := &recordingBackend{respond: func(call int, _ any) error {
				if call == 0 {
					return errBackend
				}
				return nil
			}}
			consume := newRetryHarness(t, sig, []*recordingBackend{healthy, rejecting, flaky}, testRetryConfig(5*time.Second))

			require.NoError(t, consume(t.Context(), map[string]int{"h1": 0, "p1": 1, "f1": 2, "f2": 2}))
			assert.Equal(t, [][]string{expectedItems(sig, []string{"h1"})}, healthy.calls())
			assert.Equal(t, [][]string{expectedItems(sig, []string{"p1"})}, rejecting.calls())
			assert.Equal(t, [][]string{expectedItems(sig, []string{"f1", "f2"}), expectedItems(sig, []string{"f1", "f2"})}, flaky.calls())
		})

		t.Run(sig.name()+"/retry exhaustion", func(t *testing.T) {
			healthy := &recordingBackend{}
			failing := &recordingBackend{respond: func(int, any) error { return errBackend }}
			consume := newRetryHarness(t, sig, []*recordingBackend{healthy, failing}, testRetryConfig(50*time.Millisecond))

			err := consume(t.Context(), map[string]int{"h1": 0, "f1": 1})
			require.ErrorIs(t, err, errBackend)
			assert.Equal(t, [][]string{expectedItems(sig, []string{"h1"})}, healthy.calls())
			calls := failing.calls()
			require.GreaterOrEqual(t, len(calls), 2, "the failed backend must be retried")
			for i, call := range calls {
				assert.Equal(t, expectedItems(sig, []string{"f1"}), call, "call %d of the failed backend", i)
			}
		})
	}
}

// TestExportBatchesBackendTelemetry checks that each backend attempt records one latency and
// one outcome data point, with the backend's endpoint label.
func TestExportBatchesBackendTelemetry(t *testing.T) {
	for _, sig := range partialFailureSignals {
		t.Run(sig.name(), func(t *testing.T) {
			tt := componenttest.NewTelemetry()
			defer func() { require.NoError(t, tt.Shutdown(t.Context())) }()
			tb, err := metadata.NewTelemetryBuilder(metadatatest.NewSettings(tt).TelemetrySettings)
			require.NoError(t, err)

			runs, _ := runExportBatches(t, sig, zap.NewNop(), tb, []backendSpec{
				{items: []string{"a"}},
				{items: []string{"b"}, fail: true},
				{items: []string{"c"}, fail: true, permanent: true},
			}, false)

			var outcomes []metricdata.DataPoint[int64]
			for i, run := range runs {
				attrs := run.exp.successAttr
				if i > 0 {
					attrs = run.exp.failureAttr
				}
				outcomes = append(outcomes, metricdata.DataPoint[int64]{Attributes: attrs, Value: 1})
			}
			metadatatest.AssertEqualLoadbalancerBackendOutcome(t, tt, outcomes, metricdatatest.IgnoreTimestamp())

			latency, err := tt.GetMetric("otelcol_loadbalancer_backend_latency")
			require.NoError(t, err)
			hist := latency.Data.(metricdata.Histogram[int64])
			require.Len(t, hist.DataPoints, len(runs))
			for _, dp := range hist.DataPoints {
				assert.Equal(t, uint64(1), dp.Count)
			}
		})
	}
}

// TestNestedAsyncQueueHidesLaterDeliveryFailure documents the README limitation: a backend
// with an asynchronous sending queue reports success once it has queued the data, so a later
// delivery failure never reaches the loadbalancing exporter's result.
func TestNestedAsyncQueueHidesLaterDeliveryFailure(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	release := make(chan struct{})
	delivered := make(chan error, 1)
	errDelivery := errors.New("delivery failed")

	qCfg := exporterhelper.NewDefaultQueueConfig()
	qCfg.WaitForResult = false
	qCfg.Batch = configoptional.None[exporterhelper.BatchConfig]()
	inner, err := exporterhelper.NewTraces(t.Context(), ts, &Config{}, func(context.Context, ptrace.Traces) error {
		<-release
		delivered <- errDelivery
		return errDelivery
	}, exporterhelper.WithQueue(configoptional.Some(qCfg)))
	require.NoError(t, err)
	require.NoError(t, inner.Start(t.Context(), componenttest.NewNopHost()))

	backend := newWrappedExporter(inner, endpointWithPort("queued"))
	backend.consumeWG.Add(1)
	e := &traceExporterImp{logger: ts.Logger, telemetry: tb}

	require.NoError(t, e.exportBatches(t.Context(), exporterTraces{backend: tracesSignal{}.build([]string{"a"}).(ptrace.Traces)}),
		"queue admission is the synchronous result")
	close(release)
	select {
	case err := <-delivered:
		require.ErrorIs(t, err, errDelivery)
	case <-time.After(5 * time.Second):
		t.Fatal("queued data was never delivered")
	}
	require.NoError(t, inner.Shutdown(t.Context()))
}

// TestRoutingPathsReachFailureAggregation checks that every routing key sends its batches
// through the partitioning exportBatches: a reported subset is returned as retryable typed
// data, and a permanent failure stays permanent.
func TestRoutingPathsReachFailureAggregation(t *testing.T) {
	routingKeys := map[string][]routingKey{
		"traces":  {traceIDRouting, svcRouting, attrRouting, randomnessRouting},
		"logs":    {traceIDRouting, svcRouting, resourceRouting, attrRouting},
		"metrics": {svcRouting, resourceRouting, metricNameRouting, streamIDRouting, attrRouting},
	}
	errBackend := errors.New("backend failed")
	for _, sig := range partialFailureSignals {
		for _, key := range routingKeys[sig.name()] {
			for _, permanent := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%d/permanent=%v", sig.name(), key, permanent), func(t *testing.T) {
					ts, tb := getTelemetryAssets(t)
					backend := sig.newBackend(endpointWithPort("only"), func(_ context.Context, data any) error {
						if permanent {
							return consumererror.NewPermanent(errBackend)
						}
						return sig.typedErr(errBackend, sig.filter(data, []string{"b"}))
					})
					lb := &loadBalancer{
						ring:      newHashRing([]string{"only"}),
						exporters: map[string]*wrappedExporter{endpointWithPort("only"): backend},
					}
					attrs := []string{"service.name"}

					data := sig.build([]string{"a", "b"})
					var err error
					switch d := data.(type) {
					case ptrace.Traces:
						for i := 0; i < d.ResourceSpans().Len(); i++ {
							d.ResourceSpans().At(i).Resource().Attributes().PutStr("service.name", "svc")
						}
						e := &traceExporterImp{loadBalancer: lb, routingKey: key, routingAttrs: attrs, logger: ts.Logger, telemetry: tb}
						err = e.ConsumeTraces(t.Context(), d)
					case plog.Logs:
						for i := 0; i < d.ResourceLogs().Len(); i++ {
							d.ResourceLogs().At(i).Resource().Attributes().PutStr("service.name", "svc")
						}
						e := &logExporterImp{loadBalancer: lb, routingKey: key, routingAttrs: attrs, logger: ts.Logger, telemetry: tb}
						err = e.ConsumeLogs(t.Context(), d)
					case pmetric.Metrics:
						for i := 0; i < d.ResourceMetrics().Len(); i++ {
							d.ResourceMetrics().At(i).Resource().Attributes().PutStr("service.name", "svc")
						}
						e := &metricExporterImp{loadBalancer: lb, routingKey: key, routingAttrs: attrs, logger: ts.Logger, telemetry: tb}
						err = e.ConsumeMetrics(t.Context(), d)
					}

					require.ErrorIs(t, err, errBackend)
					assert.Equal(t, permanent, consumererror.IsPermanent(err))
					got, ok := sig.typedData(err)
					require.True(t, ok, "expected a typed %s error, got %T", sig.name(), err)
					if permanent {
						assert.Equal(t, []string{"a", "b"}, itemNames(sig.items(got)))
					} else {
						assert.Equal(t, []string{"b"}, itemNames(sig.items(got)))
					}
					assert.True(t, waitGroupReturns(&backend.consumeWG))
				})
			}
		}
	}
}

// itemNames returns the item part of identities built by itemIdentity.
func itemNames(identities []string) []string {
	names := make([]string, 0, len(identities))
	for _, id := range identities {
		names = append(names, strings.Split(id, "|")[1])
	}
	sort.Strings(names)
	return names
}
