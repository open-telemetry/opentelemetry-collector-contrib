// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package adaptivetailsamplingprocessor

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/adaptivetailsamplingprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/adaptivetailsamplingprocessor/internal/metadatatest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/adaptivetailsamplingprocessor/internal/sampler"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/adaptivetailsamplingprocessor/internal/samplingstate"
)

// fakeHost supplies extensions to Start without a full collector runtime.
type fakeHost struct {
	exts map[component.ID]component.Component
}

func (h *fakeHost) GetExtensions() map[component.ID]component.Component { return h.exts }

// fakeCounterExtension satisfies samplingstate.CounterStore structurally (the way a
// real sampling-state extension would, without importing the processor) plus
// the component lifecycle.
type fakeCounterExtension struct {
	store *samplingstate.MemoryCounterStore
}

func (*fakeCounterExtension) Start(context.Context, component.Host) error { return nil }
func (*fakeCounterExtension) Shutdown(context.Context) error              { return nil }

func (e *fakeCounterExtension) AddCounts(ctx context.Context, samplerID string, bucket int64, counts map[string]float64) error {
	return e.store.AddCounts(ctx, samplerID, bucket, counts)
}

func (e *fakeCounterExtension) ReadCounts(ctx context.Context, samplerID string, bucket int64) (map[string]float64, error) {
	return e.store.ReadCounts(ctx, samplerID, bucket)
}

// notAStoreExtension has the lifecycle methods but not the counter methods.
type notAStoreExtension struct{}

func (notAStoreExtension) Start(context.Context, component.Host) error { return nil }
func (notAStoreExtension) Shutdown(context.Context) error              { return nil }

func TestResolveCounterStore(t *testing.T) {
	extID := component.MustNewID("redis_sampling_state")

	t.Run("unset config defaults to in-memory", func(t *testing.T) {
		store, err := resolveCounterStore(nil, nil)
		require.NoError(t, err)
		assert.IsType(t, &samplingstate.MemoryCounterStore{}, store)
	})

	t.Run("extension resolved structurally", func(t *testing.T) {
		host := &fakeHost{exts: map[component.ID]component.Component{
			extID: &fakeCounterExtension{store: samplingstate.NewMemoryCounterStore()},
		}}
		store, err := resolveCounterStore(host, &SharedCountersConfig{Extension: extID})
		require.NoError(t, err)
		assert.IsType(t, &fakeCounterExtension{}, store)
	})

	t.Run("missing extension errors", func(t *testing.T) {
		host := &fakeHost{exts: map[component.ID]component.Component{}}
		_, err := resolveCounterStore(host, &SharedCountersConfig{Extension: extID})
		assert.ErrorContains(t, err, "not found")
	})

	t.Run("wrong-type extension errors", func(t *testing.T) {
		host := &fakeHost{exts: map[component.ID]component.Component{
			extID: notAStoreExtension{},
		}}
		_, err := resolveCounterStore(host, &SharedCountersConfig{Extension: extID})
		assert.ErrorContains(t, err, "does not implement")
	})

	t.Run("nil host with config errors", func(t *testing.T) {
		_, err := resolveCounterStore(nil, &SharedCountersConfig{Extension: extID})
		assert.Error(t, err)
	})
}

func TestProcessorStart_SharedCountersExtensionMissing(t *testing.T) {
	cfg := &Config{
		TraceTimeout:  time.Hour,
		DecisionDelay: time.Millisecond,
		NumTraces:     10,
		Rules: []RuleConfig{
			{Name: "default", Sampler: SamplerConfig{
				Type:                  AdaptiveThroughput,
				GoalThroughput:        100,
				FingerprintAttributes: []string{`resource.attributes["service.name"]`},
				SharedCounters:        &SharedCountersConfig{Extension: component.MustNewID("redis_sampling_state")},
			}},
		},
	}
	require.NoError(t, cfg.Validate())
	p, err := newProcessor(processortest.NewNopSettings(metadata.Type), cfg, &consumertest.TracesSink{})
	require.NoError(t, err)
	err = p.Start(t.Context(), &fakeHost{exts: map[component.ID]component.Component{}})
	require.ErrorContains(t, err, "not found")
}

func TestProcessor_SharedCountersEndToEnd(t *testing.T) {
	extID := component.MustNewID("redis_sampling_state")
	ext := &fakeCounterExtension{store: samplingstate.NewMemoryCounterStore()}
	sink := &consumertest.TracesSink{}
	cfg := throughputTestConfig(&SharedCountersConfig{Extension: extID})
	require.NoError(t, cfg.Validate())
	p, err := newProcessor(processortest.NewNopSettings(metadata.Type), cfg, sink)
	require.NoError(t, err)
	require.NoError(t, p.Start(t.Context(), &fakeHost{exts: map[component.ID]component.Component{extID: ext}}))
	t.Cleanup(func() {
		require.NoError(t, p.Shutdown(context.Background())) //nolint:usetesting // cleanup after ctx cancel
	})

	// Max randomness so the trace is kept at the bootstrap rate; pins that the
	// decide path works while the sampler is wired to an extension store.
	id := [16]byte{0xAB}
	for j := 9; j < 16; j++ {
		id[j] = 0xFF
	}
	td := newRootTrace(pcommon.TraceID(id))
	td.ResourceSpans().At(0).Resource().Attributes().PutStr("service.name", "svc")
	require.NoError(t, p.ConsumeTraces(t.Context(), td))
	assert.Eventually(t, func() bool { return sink.SpanCount() == 1 }, time.Second, 10*time.Millisecond)
}

// throughputTestConfig returns a single-rule adaptive_throughput config with a
// long enough interval that the background sync loop never ticks during the
// test; syncCounters is driven by hand.
func throughputTestConfig(shared *SharedCountersConfig) *Config {
	return &Config{
		TraceTimeout:  time.Hour,
		DecisionDelay: time.Millisecond,
		NumTraces:     10,
		Rules: []RuleConfig{
			{Name: "default", Sampler: SamplerConfig{
				Type:                  AdaptiveThroughput,
				GoalThroughput:        100,
				AdjustmentInterval:    time.Hour,
				FingerprintAttributes: []string{`resource.attributes["service.name"]`},
				SharedCounters:        shared,
			}},
		},
	}
}

func TestSyncCounters_AppliesMergedCounts(t *testing.T) {
	p, err := newProcessor(processortest.NewNopSettings(metadata.Type), throughputTestConfig(nil), &consumertest.TracesSink{})
	require.NoError(t, err)
	st := p.rules[0].sampler.(*sampler.SharedThroughput)
	store := samplingstate.NewMemoryCounterStore()

	// Heavy traffic on one key: after a sync tick the rate must move off the
	// bootstrap value of 10.
	st.GetSampleRate("svc-a", 500000)
	require.Equal(t, 10, st.GetSampleRate("svc-a", 1), "bootstrap rate before the first sync")

	p.syncCounters(t.Context(), time.Unix(3600, 0), "default", st, store, time.Second)
	assert.NotEqual(t, 10, st.GetSampleRate("svc-a", 1), "sync must apply a computed table")
}

func TestSyncCounters_MergesAcrossInstances(t *testing.T) {
	sink := &consumertest.TracesSink{}
	newInstance := func() *sampler.SharedThroughput {
		p, err := newProcessor(processortest.NewNopSettings(metadata.Type), throughputTestConfig(nil), sink)
		require.NoError(t, err)
		return p.rules[0].sampler.(*sampler.SharedThroughput)
	}
	a, b := newInstance(), newInstance()
	store := samplingstate.NewMemoryCounterStore()
	pa, err := newProcessor(processortest.NewNopSettings(metadata.Type), throughputTestConfig(nil), sink)
	require.NoError(t, err)

	// Each instance sees a different key; after both publish into the shared
	// store and read back, both hold rates for both keys.
	tick := time.Unix(3600, 0)
	a.GetSampleRate("svc-a", 400000)
	b.GetSampleRate("svc-b", 400000)
	// Publish both before either reads: pin only the merged outcome, not
	// publish/read interleaving (which real instances race on).
	require.NoError(t, store.AddCounts(t.Context(), "default", 0, a.SnapshotCounts()))
	require.NoError(t, store.AddCounts(t.Context(), "default", 0, b.SnapshotCounts()))
	pa.syncCounters(t.Context(), tick, "default", a, store, time.Second)
	pa.syncCounters(t.Context(), tick, "default", b, store, time.Second)

	for _, key := range []string{"svc-a", "svc-b"} {
		assert.Equal(t, a.GetSampleRate(key, 1), b.GetSampleRate(key, 1),
			"both instances must compute identical rates for %s from the merged counts", key)
		assert.NotEqual(t, 10, a.GetSampleRate(key, 1),
			"rates for %s must come from the merged table, not bootstrap", key)
	}
}

// erroringStore fails the chosen operations.
type erroringStore struct {
	failAdd  bool
	failRead bool
	inner    *samplingstate.MemoryCounterStore
}

func (s *erroringStore) AddCounts(ctx context.Context, samplerID string, bucket int64, counts map[string]float64) error {
	if s.failAdd {
		return errors.New("add refused")
	}
	return s.inner.AddCounts(ctx, samplerID, bucket, counts)
}

func (s *erroringStore) ReadCounts(ctx context.Context, samplerID string, bucket int64) (map[string]float64, error) {
	if s.failRead {
		return nil, errors.New("read refused")
	}
	return s.inner.ReadCounts(ctx, samplerID, bucket)
}

func TestSyncCounters_FailsOpenOnStoreErrors(t *testing.T) {
	for _, tc := range []struct {
		name  string
		store *erroringStore
		op    string
	}{
		{name: "add fails", store: &erroringStore{failAdd: true, inner: samplingstate.NewMemoryCounterStore()}, op: "add"},
		{name: "read fails", store: &erroringStore{failRead: true, inner: samplingstate.NewMemoryCounterStore()}, op: "read"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tt := componenttest.NewTelemetry()
			t.Cleanup(func() {
				require.NoError(t, tt.Shutdown(context.Background())) //nolint:usetesting // cleanup after ctx cancel
			})
			p, err := newProcessor(metadatatest.NewSettings(tt), throughputTestConfig(nil), &consumertest.TracesSink{})
			require.NoError(t, err)
			st := p.rules[0].sampler.(*sampler.SharedThroughput)

			st.GetSampleRate("svc-a", 500000)
			p.syncCounters(t.Context(), time.Unix(3600, 0), "default", st, tc.store, time.Second)

			assert.NotEqual(t, 10, st.GetSampleRate("svc-a", 1),
				"store failure must fail open: local counts still produce a table")
			metadatatest.AssertEqualProcessorAdaptiveTailSamplingCounterSyncErrors(t, tt,
				[]metricdata.DataPoint[int64]{{
					Value: 1,
					Attributes: attribute.NewSet(
						attribute.String("rule", "default"),
						attribute.String("op", tc.op),
					),
				}},
				metricdatatest.IgnoreTimestamp(),
				metricdatatest.IgnoreExemplars(),
			)
		})
	}
}

// blockingStore hangs on the chosen operation until its context is cancelled,
// standing in for a store slower than the sync timeout.
type blockingStore struct {
	blockAdd  bool
	blockRead bool
	inner     *samplingstate.MemoryCounterStore
}

func (s *blockingStore) AddCounts(ctx context.Context, samplerID string, bucket int64, counts map[string]float64) error {
	if s.blockAdd {
		<-ctx.Done()
		return ctx.Err()
	}
	return s.inner.AddCounts(ctx, samplerID, bucket, counts)
}

func (s *blockingStore) ReadCounts(ctx context.Context, samplerID string, bucket int64) (map[string]float64, error) {
	if s.blockRead {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return s.inner.ReadCounts(ctx, samplerID, bucket)
}

func TestSyncCounters_TimeoutFailsOpen(t *testing.T) {
	for _, tc := range []struct {
		name  string
		store *blockingStore
		op    string
	}{
		{name: "add times out", store: &blockingStore{blockAdd: true, inner: samplingstate.NewMemoryCounterStore()}, op: "add"},
		{name: "read times out", store: &blockingStore{blockRead: true, inner: samplingstate.NewMemoryCounterStore()}, op: "read"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tt := componenttest.NewTelemetry()
			t.Cleanup(func() {
				require.NoError(t, tt.Shutdown(context.Background())) //nolint:usetesting // cleanup after ctx cancel
			})
			p, err := newProcessor(metadatatest.NewSettings(tt), throughputTestConfig(nil), &consumertest.TracesSink{})
			require.NoError(t, err)
			st := p.rules[0].sampler.(*sampler.SharedThroughput)

			st.GetSampleRate("svc-a", 500000)
			done := make(chan struct{})
			go func() {
				// A hung store must not stall the caller past the timeout.
				p.syncCounters(t.Context(), time.Unix(3600, 0), "default", st, tc.store, 20*time.Millisecond)
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("syncCounters did not return; timeout was not enforced")
			}

			assert.NotEqual(t, 10, st.GetSampleRate("svc-a", 1),
				"a store that times out must fail open to local counts")
			metadatatest.AssertEqualProcessorAdaptiveTailSamplingCounterSyncErrors(t, tt,
				[]metricdata.DataPoint[int64]{{
					Value: 1,
					Attributes: attribute.NewSet(
						attribute.String("rule", "default"),
						attribute.String("op", tc.op),
					),
				}},
				metricdatatest.IgnoreTimestamp(),
				metricdatatest.IgnoreExemplars(),
			)
		})
	}
}

func TestNextBoundary(t *testing.T) {
	interval := 15 * time.Second
	now := time.Unix(100, 500)
	next := nextBoundary(now, interval)
	assert.Equal(t, time.Unix(105, 0), next)
	assert.Equal(t, time.Unix(120, 0), nextBoundary(next, interval),
		"a tick exactly on a boundary schedules the following boundary, not itself")
}
