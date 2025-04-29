// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tailsamplingprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/metadata"
)

func TestMetricsAfterOneEvaluation(t *testing.T) {
	// prepare
	s := setupTestTelemetry()
	b := newSyncIDBatcher()
	syncBatcher := b.(*syncIDBatcher)

	cfg := Config{
		DecisionWait: 1,
		NumTraces:    100,
		PolicyCfgs: []PolicyCfg{
			{
				sharedPolicyCfg: sharedPolicyCfg{
					Name: "always",
					Type: AlwaysSample,
				},
			},
		},
		Options: []Option{
			withDecisionBatcher(syncBatcher),
		},
	}
	cs := &consumertest.TracesSink{}
	ct := s.newSettings()
	proc, err := newTracesProcessor(context.Background(), ct, cs, cfg)
	require.NoError(t, err)
	defer func() {
		err = proc.Shutdown(context.Background())
		require.NoError(t, err)
	}()

	err = proc.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	// test
	err = proc.ConsumeTraces(context.Background(), simpleTraces())
	require.NoError(t, err)

	tsp := proc.(*tailSamplingSpanProcessor)
	tsp.policyTicker.OnTick() // the first tick always gets an empty batch
	tsp.policyTicker.OnTick()

	// verify
	var md metricdata.ResourceMetrics
	require.NoError(t, s.reader.Collect(context.Background(), &md))
	require.Equal(t, 8, s.len(md))

	for _, tt := range []struct {
		opts []metricdatatest.Option
		m    metricdata.Metrics
	}{
		{
			opts: []metricdatatest.Option{metricdatatest.IgnoreTimestamp()},
			m: metricdata.Metrics{
				Name:        "otelcol_processor_tail_sampling_count_traces_sampled",
				Description: "Count of traces that were sampled or not per sampling policy",
				Unit:        "{traces}",
				Data: metricdata.Sum[int64]{
					IsMonotonic: true,
					Temporality: metricdata.CumulativeTemporality,
					DataPoints: []metricdata.DataPoint[int64]{
						{
							Attributes: attribute.NewSet(
								attribute.String("policy", "always"),
								attribute.String("sampled", "true"),
							),
							Value: 1,
						},
					},
				},
			},
		},
		{
			opts: []metricdatatest.Option{metricdatatest.IgnoreTimestamp()},
			m: metricdata.Metrics{
				Name:        "otelcol_processor_tail_sampling_global_count_traces_sampled",
				Description: "Global count of traces that were sampled or not by at least one policy",
				Unit:        "{traces}",
				Data: metricdata.Sum[int64]{
					IsMonotonic: true,
					Temporality: metricdata.CumulativeTemporality,
					DataPoints: []metricdata.DataPoint[int64]{
						{
							Attributes: attribute.NewSet(
								attribute.String("sampled", "true"),
							),
							Value: 1,
						},
					},
				},
			},
		},
		{
			opts: []metricdatatest.Option{metricdatatest.IgnoreTimestamp(), metricdatatest.IgnoreValue()},
			m: metricdata.Metrics{
				Name:        "otelcol_processor_tail_sampling_sampling_decision_latency",
				Description: "Latency (in microseconds) of a given sampling policy",
				Unit:        "µs",
				Data: metricdata.Histogram[int64]{
					Temporality: metricdata.CumulativeTemporality,
					DataPoints: []metricdata.HistogramDataPoint[int64]{
						{
							Attributes: attribute.NewSet(
								attribute.String("policy", "always"),
							),
						},
					},
				},
			},
		},
		{
			opts: []metricdatatest.Option{metricdatatest.IgnoreTimestamp(), metricdatatest.IgnoreValue()},
			m: metricdata.Metrics{
				Name:        "otelcol_processor_tail_sampling_sampling_decision_timer_latency",
				Description: "Latency (in milliseconds) of each run of the sampling decision timer",
				Unit:        "ms",
				Data: metricdata.Histogram[int64]{
					Temporality: metricdata.CumulativeTemporality,
					DataPoints:  []metricdata.HistogramDataPoint[int64]{{}},
				},
			},
		},
		{
			opts: []metricdatatest.Option{metricdatatest.IgnoreTimestamp()},
			m: metricdata.Metrics{
				Name:        "otelcol_processor_tail_sampling_new_trace_id_received",
				Description: "Counts the arrival of new traces",
				Unit:        "{traces}",
				Data: metricdata.Sum[int64]{
					IsMonotonic: true,
					Temporality: metricdata.CumulativeTemporality,
					DataPoints: []metricdata.DataPoint[int64]{
						{
							Value: 1,
						},
					},
				},
			},
		},
		{
			opts: []metricdatatest.Option{metricdatatest.IgnoreTimestamp()},
			m: metricdata.Metrics{
				Name:        "otelcol_processor_tail_sampling_sampling_policy_evaluation_error",
				Description: "Count of sampling policy evaluation errors",
				Unit:        "{errors}",
				Data: metricdata.Sum[int64]{
					IsMonotonic: true,
					Temporality: metricdata.CumulativeTemporality,
					DataPoints: []metricdata.DataPoint[int64]{
						{
							Value: 0,
						},
					},
				},
			},
		},
		{
			opts: []metricdatatest.Option{metricdatatest.IgnoreTimestamp()},
			m: metricdata.Metrics{
				Name:        "otelcol_processor_tail_sampling_sampling_trace_dropped_too_early",
				Description: "Count of traces that needed to be dropped before the configured wait time",
				Unit:        "{traces}",
				Data: metricdata.Sum[int64]{
					IsMonotonic: true,
					Temporality: metricdata.CumulativeTemporality,
					DataPoints: []metricdata.DataPoint[int64]{
						{
							Value: 0,
						},
					},
				},
			},
		},
		{
			opts: []metricdatatest.Option{metricdatatest.IgnoreTimestamp()},
			m: metricdata.Metrics{
				Name:        "otelcol_processor_tail_sampling_sampling_traces_on_memory",
				Description: "Tracks the number of traces current on memory",
				Unit:        "{traces}",
				Data: metricdata.Gauge[int64]{
					DataPoints: []metricdata.DataPoint[int64]{
						{
							Value: 1,
						},
					},
				},
			},
		},
	} {
		got := s.getMetric(tt.m.Name, md)
		metricdatatest.AssertEqual(t, tt.m, got, tt.opts...)
	}

	// sanity check
	assert.Len(t, cs.AllTraces(), 1)
}

func TestMetricsWithComponentID(t *testing.T) {
	// prepare
	s := setupTestTelemetry()
	b := newSyncIDBatcher()
	syncBatcher := b.(*syncIDBatcher)

	cfg := Config{
		DecisionWait: 1,
		NumTraces:    100,
		PolicyCfgs: []PolicyCfg{
			{
				sharedPolicyCfg: sharedPolicyCfg{
					Name: "always",
					Type: AlwaysSample,
				},
			},
		},
		Options: []Option{
			withDecisionBatcher(syncBatcher),
		},
	}
	cs := &consumertest.TracesSink{}
	ct := s.newSettings()
	ct.ID = component.MustNewIDWithName("tail_sampling", "unique_id") // e.g tail_sampling/unique_id
	proc, err := newTracesProcessor(context.Background(), ct, cs, cfg)
	require.NoError(t, err)
	defer func() {
		err = proc.Shutdown(context.Background())
		require.NoError(t, err)
	}()

	err = proc.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	// test
	err = proc.ConsumeTraces(context.Background(), simpleTraces())
	require.NoError(t, err)

	tsp := proc.(*tailSamplingSpanProcessor)
	tsp.policyTicker.OnTick() // the first tick always gets an empty batch
	tsp.policyTicker.OnTick()

	// verify
	var md metricdata.ResourceMetrics
	require.NoError(t, s.reader.Collect(context.Background(), &md))
	require.Equal(t, 8, s.len(md))

	for _, tt := range []struct {
		opts []metricdatatest.Option
		m    metricdata.Metrics
	}{
		{
			opts: []metricdatatest.Option{metricdatatest.IgnoreTimestamp()},
			m: metricdata.Metrics{
				Name:        "otelcol_processor_tail_sampling_count_traces_sampled",
				Description: "Count of traces that were sampled or not per sampling policy",
				Unit:        "{traces}",
				Data: metricdata.Sum[int64]{
					IsMonotonic: true,
					Temporality: metricdata.CumulativeTemporality,
					DataPoints: []metricdata.DataPoint[int64]{
						{
							Attributes: attribute.NewSet(
								attribute.String("policy", "unique_id.always"),
								attribute.String("sampled", "true"),
							),
							Value: 1,
						},
					},
				},
			},
		},
		{
			opts: []metricdatatest.Option{metricdatatest.IgnoreTimestamp(), metricdatatest.IgnoreValue()},
			m: metricdata.Metrics{
				Name:        "otelcol_processor_tail_sampling_sampling_decision_latency",
				Description: "Latency (in microseconds) of a given sampling policy",
				Unit:        "µs",
				Data: metricdata.Histogram[int64]{
					Temporality: metricdata.CumulativeTemporality,
					DataPoints: []metricdata.HistogramDataPoint[int64]{
						{
							Attributes: attribute.NewSet(
								attribute.String("policy", "unique_id.always"),
							),
						},
					},
				},
			},
		},
	} {
		got := s.getMetric(tt.m.Name, md)
		metricdatatest.AssertEqual(t, tt.m, got, tt.opts...)
	}

	// sanity check
	assert.Len(t, cs.AllTraces(), 1)
}

func TestProcessorTailSamplingCountSpansSampled(t *testing.T) {
	err := featuregate.GlobalRegistry().Set("processor.tailsamplingprocessor.metricstatcountspanssampled", true)
	require.NoError(t, err)

	defer func() {
		err = featuregate.GlobalRegistry().Set("processor.tailsamplingprocessor.metricstatcountspanssampled", false)
		require.NoError(t, err)
	}()

	// prepare
	s := setupTestTelemetry()
	b := newSyncIDBatcher()
	syncBatcher := b.(*syncIDBatcher)

	cfg := Config{
		DecisionWait: 1,
		NumTraces:    100,
		PolicyCfgs: []PolicyCfg{
			{
				sharedPolicyCfg: sharedPolicyCfg{
					Name: "always",
					Type: AlwaysSample,
				},
			},
		},
		Options: []Option{
			withDecisionBatcher(syncBatcher),
		},
	}
	cs := &consumertest.TracesSink{}
	ct := s.newSettings()
	proc, err := newTracesProcessor(context.Background(), ct, cs, cfg)
	require.NoError(t, err)
	defer func() {
		err = proc.Shutdown(context.Background())
		require.NoError(t, err)
	}()

	err = proc.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	// test
	err = proc.ConsumeTraces(context.Background(), simpleTraces())
	require.NoError(t, err)

	tsp := proc.(*tailSamplingSpanProcessor)
	tsp.policyTicker.OnTick() // the first tick always gets an empty batch
	tsp.policyTicker.OnTick()

	// verify
	var md metricdata.ResourceMetrics
	require.NoError(t, s.reader.Collect(context.Background(), &md))
	require.Equal(t, 9, s.len(md))

	m := metricdata.Metrics{
		Name:        "otelcol_processor_tail_sampling_count_spans_sampled",
		Description: "Count of spans that were sampled or not per sampling policy",
		Unit:        "{spans}",
		Data: metricdata.Sum[int64]{
			Temporality: metricdata.CumulativeTemporality,
			IsMonotonic: true,
			DataPoints: []metricdata.DataPoint[int64]{
				{
					Attributes: attribute.NewSet(
						attribute.String("policy", "always"),
						attribute.String("sampled", "true"),
					),
					Value: 1,
				},
			},
		},
	}
	got := s.getMetric(m.Name, md)
	metricdatatest.AssertEqual(t, m, got, metricdatatest.IgnoreTimestamp())
}

func TestProcessorTailSamplingSamplingTraceRemovalAge(t *testing.T) {
	// prepare
	s := setupTestTelemetry()
	b := newSyncIDBatcher()
	syncBatcher := b.(*syncIDBatcher)

	cfg := Config{
		DecisionWait: 1,
		NumTraces:    2,
		PolicyCfgs: []PolicyCfg{
			{
				sharedPolicyCfg: sharedPolicyCfg{
					Name: "always",
					Type: AlwaysSample,
				},
			},
		},
		Options: []Option{
			withDecisionBatcher(syncBatcher),
		},
	}
	cs := &consumertest.TracesSink{}
	ct := s.newSettings()
	proc, err := newTracesProcessor(context.Background(), ct, cs, cfg)
	require.NoError(t, err)
	defer func() {
		err = proc.Shutdown(context.Background())
		require.NoError(t, err)
	}()

	err = proc.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	// test
	_, batches := generateIDsAndBatches(3)
	for _, batch := range batches {
		err = proc.ConsumeTraces(context.Background(), batch)
		require.NoError(t, err)
	}

	tsp := proc.(*tailSamplingSpanProcessor)
	tsp.policyTicker.OnTick() // the first tick always gets an empty batch
	tsp.policyTicker.OnTick()

	// verify
	var md metricdata.ResourceMetrics
	require.NoError(t, s.reader.Collect(context.Background(), &md))

	m := metricdata.Metrics{
		Name:        "otelcol_processor_tail_sampling_sampling_trace_removal_age",
		Description: "Time (in seconds) from arrival of a new trace until its removal from memory",
		Unit:        "s",
		Data: metricdata.Histogram[int64]{
			Temporality: metricdata.CumulativeTemporality,
			DataPoints:  []metricdata.HistogramDataPoint[int64]{{}},
		},
	}
	got := s.getMetric(m.Name, md)
	metricdatatest.AssertEqual(t, m, got, metricdatatest.IgnoreTimestamp(), metricdatatest.IgnoreValue())
}

func TestProcessorTailSamplingSamplingLateSpanAge(t *testing.T) {
	// prepare
	s := setupTestTelemetry()
	b := newSyncIDBatcher()
	syncBatcher := b.(*syncIDBatcher)

	cfg := Config{
		DecisionWait: 1,
		NumTraces:    100,
		PolicyCfgs: []PolicyCfg{
			{
				sharedPolicyCfg: sharedPolicyCfg{
					Name: "sample-half",
					Type: Probabilistic,
					ProbabilisticCfg: ProbabilisticCfg{
						SamplingPercentage: 50,
					},
				},
			},
		},
		Options: []Option{
			withDecisionBatcher(syncBatcher),
		},
	}
	cs := &consumertest.TracesSink{}
	ct := s.newSettings()
	proc, err := newTracesProcessor(context.Background(), ct, cs, cfg)
	require.NoError(t, err)
	defer func() {
		err = proc.Shutdown(context.Background())
		require.NoError(t, err)
	}()

	err = proc.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	// test
	traceIDs, batches := generateIDsAndBatches(10)
	for _, batch := range batches {
		err = proc.ConsumeTraces(context.Background(), batch)
		require.NoError(t, err)
	}

	tsp := proc.(*tailSamplingSpanProcessor)
	tsp.policyTicker.OnTick() // the first tick always gets an empty batch
	tsp.policyTicker.OnTick()

	for _, traceID := range traceIDs {
		lateSpan := ptrace.NewTraces()
		lateSpan.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetTraceID(traceID)

		err = proc.ConsumeTraces(context.Background(), lateSpan)
		require.NoError(t, err)
	}

	// verify
	var md metricdata.ResourceMetrics
	require.NoError(t, s.reader.Collect(context.Background(), &md))

	m := metricdata.Metrics{
		Name:        "otelcol_processor_tail_sampling_sampling_late_span_age",
		Description: "Time (in seconds) from the sampling decision was taken and the arrival of a late span",
		Unit:        "s",
		Data: metricdata.Histogram[int64]{
			Temporality: metricdata.CumulativeTemporality,
			DataPoints: []metricdata.HistogramDataPoint[int64]{
				{
					Count:        10,
					Bounds:       []float64{0, 5, 10, 25, 50, 75, 100, 250, 500, 750, 1000, 2500, 5000, 7500, 10000},
					BucketCounts: []uint64{10, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
					Min:          metricdata.NewExtrema[int64](0),
					Max:          metricdata.NewExtrema[int64](0),
					Sum:          0,
				},
			},
		},
	}

	got := s.getMetric(m.Name, md)

	metricdatatest.AssertEqual(t, m, got, metricdatatest.IgnoreTimestamp())
}

func TestProcessorTailSamplingSamplingTraceDroppedTooEarly(t *testing.T) {
	// prepare
	s := setupTestTelemetry()
	b := newSyncIDBatcher()
	syncBatcher := b.(*syncIDBatcher)

	cfg := Config{
		DecisionWait: 1,
		NumTraces:    2,
		PolicyCfgs: []PolicyCfg{
			{
				sharedPolicyCfg: sharedPolicyCfg{
					Name: "always",
					Type: AlwaysSample,
				},
			},
		},
		Options: []Option{
			withDecisionBatcher(syncBatcher),
		},
	}
	cs := &consumertest.TracesSink{}
	ct := s.newSettings()
	proc, err := newTracesProcessor(context.Background(), ct, cs, cfg)
	require.NoError(t, err)
	defer func() {
		err = proc.Shutdown(context.Background())
		require.NoError(t, err)
	}()

	err = proc.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	// test
	_, batches := generateIDsAndBatches(3)
	for _, batch := range batches {
		err = proc.ConsumeTraces(context.Background(), batch)
		require.NoError(t, err)
	}

	tsp := proc.(*tailSamplingSpanProcessor)
	tsp.policyTicker.OnTick() // the first tick always gets an empty batch
	tsp.policyTicker.OnTick()

	// verify
	var md metricdata.ResourceMetrics
	require.NoError(t, s.reader.Collect(context.Background(), &md))

	m := metricdata.Metrics{
		Name:        "otelcol_processor_tail_sampling_sampling_trace_dropped_too_early",
		Description: "Count of traces that needed to be dropped before the configured wait time",
		Unit:        "{traces}",
		Data: metricdata.Sum[int64]{
			IsMonotonic: true,
			Temporality: metricdata.CumulativeTemporality,
			DataPoints: []metricdata.DataPoint[int64]{
				{
					Value: 1,
				},
			},
		},
	}

	got := s.getMetric(m.Name, md)
	metricdatatest.AssertEqual(t, m, got, metricdatatest.IgnoreTimestamp())
}

func TestProcessorTailSamplingSamplingPolicyEvaluationError(t *testing.T) {
	// prepare
	s := setupTestTelemetry()
	b := newSyncIDBatcher()
	syncBatcher := b.(*syncIDBatcher)

	cfg := Config{
		DecisionWait: 1,
		NumTraces:    100,
		PolicyCfgs: []PolicyCfg{
			{
				sharedPolicyCfg: sharedPolicyCfg{
					Name: "ottl",
					Type: OTTLCondition,
					OTTLConditionCfg: OTTLConditionCfg{
						ErrorMode:      ottl.PropagateError,
						SpanConditions: []string{"attributes[1] == \"test\""},
					},
				},
			},
		},
		Options: []Option{
			withDecisionBatcher(syncBatcher),
		},
	}
	cs := &consumertest.TracesSink{}
	ct := s.newSettings()
	proc, err := newTracesProcessor(context.Background(), ct, cs, cfg)
	require.NoError(t, err)
	defer func() {
		err = proc.Shutdown(context.Background())
		require.NoError(t, err)
	}()

	err = proc.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	// test
	_, batches := generateIDsAndBatches(2)
	for _, batch := range batches {
		err = proc.ConsumeTraces(context.Background(), batch)
		require.NoError(t, err)
	}

	tsp := proc.(*tailSamplingSpanProcessor)
	tsp.policyTicker.OnTick() // the first tick always gets an empty batch
	tsp.policyTicker.OnTick()

	// verify
	var md metricdata.ResourceMetrics
	require.NoError(t, s.reader.Collect(context.Background(), &md))

	m := metricdata.Metrics{
		Name:        "otelcol_processor_tail_sampling_sampling_policy_evaluation_error",
		Description: "Count of sampling policy evaluation errors",
		Unit:        "{errors}",
		Data: metricdata.Sum[int64]{
			IsMonotonic: true,
			Temporality: metricdata.CumulativeTemporality,
			DataPoints: []metricdata.DataPoint[int64]{
				{
					Value: 2,
				},
			},
		},
	}

	got := s.getMetric(m.Name, md)
	metricdatatest.AssertEqual(t, m, got, metricdatatest.IgnoreTimestamp())
}

type testTelemetry struct {
	reader        *sdkmetric.ManualReader
	meterProvider *sdkmetric.MeterProvider
}

func setupTestTelemetry() testTelemetry {
	reader := sdkmetric.NewManualReader()
	return testTelemetry{
		reader:        reader,
		meterProvider: sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader)),
	}
}

func (tt *testTelemetry) newSettings() processor.Settings {
	set := processortest.NewNopSettings(metadata.Type)
	set.ID = component.NewID(component.MustNewType("tail_sampling"))
	set.MeterProvider = tt.meterProvider
	return set
}

func (tt *testTelemetry) getMetric(name string, got metricdata.ResourceMetrics) metricdata.Metrics {
	for _, sm := range got.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == name {
				return m
			}
		}
	}

	return metricdata.Metrics{}
}

func (tt *testTelemetry) len(got metricdata.ResourceMetrics) int {
	metricsCount := 0
	for _, sm := range got.ScopeMetrics {
		metricsCount += len(sm.Metrics)
	}

	return metricsCount
}

func (tt *testTelemetry) Shutdown(ctx context.Context) error {
	return tt.meterProvider.Shutdown(ctx)
}
