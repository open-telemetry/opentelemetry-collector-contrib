// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package deltatocumulativeprocessor

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/maps"
)

type storageHost struct{ client storage.Client }

func (h storageHost) GetExtensions() map[component.ID]component.Component {
	return map[component.ID]component.Component{component.MustNewID("file_storage"): storageExtension{client: h.client}}
}

type storageExtension struct {
	extension.Extension
	client storage.Client
}

func (e storageExtension) GetClient(context.Context, component.Kind, component.ID, string) (storage.Client, error) {
	return e.client, nil
}

func storageProcessor(t *testing.T) *deltaToCumulativeProcessor {
	t.Helper()
	cfg := createDefaultConfig().(*Config)
	cfg.StorageID = new(component.MustNewID("file_storage"))
	proc, _ := setup(t, cfg, &consumertest.MetricsSink{})
	return proc.(*deltaToCumulativeProcessor)
}

type lifecycleClient struct {
	*memClient
	block   bool
	entered atomic.Bool
	closed  atomic.Bool
}

func (c *lifecycleClient) Batch(ctx context.Context, ops ...*storage.Operation) error {
	if c.block && c.entered.CompareAndSwap(false, true) {
		<-ctx.Done()
		return ctx.Err()
	}
	return c.memClient.Batch(ctx, ops...)
}
func (c *lifecycleClient) Close(context.Context) error { c.closed.Store(true); return nil }

func TestStorageStartFailureShutdown(t *testing.T) {
	p := storageProcessor(t)
	require.Error(t, p.Start(t.Context(), nil))
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	require.NoError(t, p.Shutdown(ctx))
}

func TestStorageShutdownCancelsFlushAndCloses(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := &lifecycleClient{memClient: newMemClient(), block: true}
		p := storageProcessor(t)
		require.NoError(t, p.Start(t.Context(), storageHost{client: c}))
		dp := pmetric.NewNumberDataPoint()
		dp.SetIntValue(42)
		_, _, err := p.last.nums.LoadOrStore(t.Context(), streamID{}, guard(dp))
		require.NoError(t, err)
		synctest.Wait()
		time.Sleep(time.Minute)
		synctest.Wait()
		require.True(t, c.entered.Load())
		require.NoError(t, p.Shutdown(t.Context()))
		require.True(t, c.closed.Load())
		data, err := c.Get(t.Context(), "nums/"+streamHashKey(streamID{}))
		require.NoError(t, err)
		recovered, err := unmarshalNumberDP(data)
		require.NoError(t, err)
		require.Equal(t, int64(42), recovered.v.IntValue())
	})
}

type deleteClient struct {
	*memClient
	fail bool
}

func (c *deleteClient) Delete(ctx context.Context, key string) error {
	if c.fail {
		return errors.New("delete failed")
	}
	return c.memClient.Delete(ctx, key)
}

func TestStorageOrphansSurviveRestartsAndDeleteFailures(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := t.Context()
		client := &deleteClient{memClient: newMemClient()}
		require.NoError(t, client.Set(ctx, "nums/orphan", []byte("old state")))
		require.NoError(t, persistStaleIndex(ctx, client, []string{"orphan"}))
		// Shut down before the cleanup grace period, twice.
		for range 2 {
			state, err := newPersistedState(ctx, client, maps.Limit(10), 5*time.Minute)
			require.NoError(t, err)
			require.NoError(t, state.flush(ctx))
			require.Contains(t, readStaleIndex(t, client), "orphan")
		}
		state, err := newPersistedState(ctx, client, maps.Limit(10), 5*time.Minute)
		require.NoError(t, err)
		time.Sleep(5 * time.Minute)
		client.fail = true
		require.Error(t, state.flush(ctx))
		require.Contains(t, readStaleIndex(t, client), "orphan")
		client.fail = false
		require.NoError(t, state.flush(ctx))
		require.Empty(t, readStaleIndex(t, client))
		data, err := client.Get(ctx, "nums/orphan")
		require.NoError(t, err)
		require.Empty(t, data)
	})
}

func readStaleIndex(t *testing.T, client storage.Client) map[string]struct{} {
	t.Helper()
	keys, err := loadStaleIndex(t.Context(), client)
	require.NoError(t, err)
	return keys
}

type readFailureClient struct {
	*lifecycleClient
	failIndex bool
}

func (c *readFailureClient) Get(ctx context.Context, key string) ([]byte, error) {
	if c.failIndex || key != staleIndexKey {
		return nil, errors.New("storage unavailable")
	}
	return c.memClient.Get(ctx, key)
}

func TestStorageIndexFailureClosesClient(t *testing.T) {
	client := &readFailureClient{lifecycleClient: &lifecycleClient{memClient: newMemClient()}, failIndex: true}
	p := storageProcessor(t)
	require.ErrorContains(t, p.Start(t.Context(), storageHost{client: client}), "load stale index")
	require.True(t, client.closed.Load())
	require.NoError(t, p.Shutdown(t.Context()))
}

func TestStorageReadFailureIsReported(t *testing.T) {
	client := &readFailureClient{lifecycleClient: &lifecycleClient{memClient: newMemClient()}}
	p := storageProcessor(t)
	require.NoError(t, p.Start(t.Context(), storageHost{client: client}))
	md := pmetric.NewMetrics()
	sum := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetEmptySum()
	sum.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	sum.DataPoints().AppendEmpty().SetIntValue(42)
	err := p.ConsumeMetrics(t.Context(), md)
	require.ErrorContains(t, err, "storage unavailable")
	require.True(t, consumererror.IsPermanent(err))
	require.Zero(t, p.last.Size())
	require.NoError(t, p.Shutdown(t.Context()))
}

func TestStorageTracksActiveStreams(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.StorageID = new(component.MustNewID("file_storage"))
	proc, telemetry := setup(t, cfg, &consumertest.MetricsSink{})
	p := proc.(*deltaToCumulativeProcessor)
	require.NoError(t, p.Start(t.Context(), storageHost{client: newMemClient()}))
	defer func() { require.NoError(t, p.Shutdown(t.Context())) }()
	_, _, err := p.last.nums.LoadOrStore(t.Context(), streamID{}, guard(pmetric.NewNumberDataPoint()))
	require.NoError(t, err)
	var rm metricdata.ResourceMetrics
	require.NoError(t, telemetry.reader.Collect(t.Context(), &rm))
	for _, scope := range rm.ScopeMetrics {
		for _, metric := range scope.Metrics {
			if metric.Name != "otelcol_deltatocumulative_streams_tracked" {
				continue
			}
			sum, ok := metric.Data.(metricdata.Sum[int64])
			require.True(t, ok)
			require.Len(t, sum.DataPoints, 1)
			require.Equal(t, int64(1), sum.DataPoints[0].Value)
			return
		}
	}
	t.Fatal("streams tracked metric missing")
}

func deltaMetrics(typ pmetric.MetricType, start, end pcommon.Timestamp, value int64) pmetric.Metrics {
	md := pmetric.NewMetrics()
	m := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m.SetName("requests")
	switch typ {
	case pmetric.MetricTypeSum:
		sum := m.SetEmptySum()
		sum.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		sum.SetIsMonotonic(true)
		dp := sum.DataPoints().AppendEmpty()
		dp.SetStartTimestamp(start)
		dp.SetTimestamp(end)
		dp.SetIntValue(value)
	case pmetric.MetricTypeHistogram:
		hist := m.SetEmptyHistogram()
		hist.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		dp := hist.DataPoints().AppendEmpty()
		dp.SetStartTimestamp(start)
		dp.SetTimestamp(end)
		dp.SetCount(1)
		dp.SetSum(float64(value))
		dp.ExplicitBounds().FromRaw([]float64{10})
		dp.BucketCounts().FromRaw([]uint64{1, 0})
	case pmetric.MetricTypeExponentialHistogram:
		hist := m.SetEmptyExponentialHistogram()
		hist.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		dp := hist.DataPoints().AppendEmpty()
		dp.SetStartTimestamp(start)
		dp.SetTimestamp(end)
		dp.SetCount(1)
		dp.SetSum(float64(value))
		dp.Positive().SetOffset(1)
		dp.Positive().BucketCounts().FromRaw([]uint64{1})
	}
	return md
}

func TestStorageProcessorRecoversAllMetricTypes(t *testing.T) {
	for _, typ := range []pmetric.MetricType{pmetric.MetricTypeSum, pmetric.MetricTypeHistogram, pmetric.MetricTypeExponentialHistogram} {
		t.Run(typ.String(), func(t *testing.T) {
			client := newMemClient()
			first := storageProcessor(t)
			require.NoError(t, first.Start(t.Context(), storageHost{client: client}))
			require.NoError(t, first.ConsumeMetrics(t.Context(), deltaMetrics(typ, 1, 2, 2)))
			require.NoError(t, first.Shutdown(t.Context()))
			second := storageProcessor(t)
			require.NoError(t, second.Start(t.Context(), storageHost{client: client}))
			require.NoError(t, second.ConsumeMetrics(t.Context(), deltaMetrics(typ, 2, 3, 3)))
			require.NoError(t, second.Shutdown(t.Context()))
			output := second.next.(*consumertest.MetricsSink).AllMetrics()
			require.Len(t, output, 1)
			m := output[0].ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
			switch typ {
			case pmetric.MetricTypeSum:
				require.Equal(t, pmetric.AggregationTemporalityCumulative, m.Sum().AggregationTemporality())
				dp := m.Sum().DataPoints().At(0)
				require.Equal(t, int64(5), dp.IntValue())
				require.Equal(t, pcommon.Timestamp(1), dp.StartTimestamp())
				require.Equal(t, pcommon.Timestamp(3), dp.Timestamp())
			case pmetric.MetricTypeHistogram:
				require.Equal(t, pmetric.AggregationTemporalityCumulative, m.Histogram().AggregationTemporality())
				dp := m.Histogram().DataPoints().At(0)
				require.Equal(t, uint64(2), dp.Count())
				require.Equal(t, float64(5), dp.Sum())
				require.Equal(t, []uint64{2, 0}, dp.BucketCounts().AsRaw())
				require.Equal(t, pcommon.Timestamp(1), dp.StartTimestamp())
				require.Equal(t, pcommon.Timestamp(3), dp.Timestamp())
			case pmetric.MetricTypeExponentialHistogram:
				require.Equal(t, pmetric.AggregationTemporalityCumulative, m.ExponentialHistogram().AggregationTemporality())
				dp := m.ExponentialHistogram().DataPoints().At(0)
				require.Equal(t, uint64(2), dp.Count())
				require.Equal(t, float64(5), dp.Sum())
				require.Equal(t, []uint64{2}, dp.Positive().BucketCounts().AsRaw())
				require.Equal(t, pcommon.Timestamp(1), dp.StartTimestamp())
				require.Equal(t, pcommon.Timestamp(3), dp.Timestamp())
			}
		})
	}
}

func TestStorageConcurrentFlushPreservesFinalValue(t *testing.T) {
	client := newMemClient()
	p := storageProcessor(t)
	require.NoError(t, p.Start(t.Context(), storageHost{client: client}))
	done := make(chan error, 1)
	go func() {
		for range 100 {
			if err := p.last.flush(t.Context()); err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}()
	for i := range 100 {
		require.NoError(t, p.ConsumeMetrics(t.Context(), deltaMetrics(pmetric.MetricTypeSum, pcommon.Timestamp(i+1), pcommon.Timestamp(i+2), 1)))
	}
	require.NoError(t, <-done)
	require.NoError(t, p.Shutdown(t.Context()))
	next := storageProcessor(t)
	require.NoError(t, next.Start(t.Context(), storageHost{client: client}))
	require.NoError(t, next.ConsumeMetrics(t.Context(), deltaMetrics(pmetric.MetricTypeSum, 101, 102, 1)))
	require.NoError(t, next.Shutdown(t.Context()))
	output := next.next.(*consumertest.MetricsSink).AllMetrics()
	require.Len(t, output, 1)
	require.Equal(t, int64(101), output[0].ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).IntValue())
}

func TestStorageKeepsCollidingStreamsSeparateAcrossRestart(t *testing.T) {
	for _, tt := range []struct {
		name      string
		configure func(pmetric.ScopeMetrics, pmetric.Metric, bool)
	}{
		{"metric boundary", func(_ pmetric.ScopeMetrics, m pmetric.Metric, second bool) {
			m.SetName("foo")
			m.SetUnit("ms")
			if second {
				m.SetName("foom")
				m.SetUnit("s")
			}
		}},
		{"scope boundary", func(sm pmetric.ScopeMetrics, _ pmetric.Metric, second bool) {
			sm.Scope().SetName("ab")
			sm.Scope().SetVersion("c")
			if second {
				sm.Scope().SetName("a")
				sm.Scope().SetVersion("bc")
			}
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			makeInput := func(second bool, start, end pcommon.Timestamp, value int64) pmetric.Metrics {
				md := deltaMetrics(pmetric.MetricTypeSum, start, end, value)
				sm := md.ResourceMetrics().At(0).ScopeMetrics().At(0)
				tt.configure(sm, sm.Metrics().At(0), second)
				return md
			}
			client := newMemClient()
			first := storageProcessor(t)
			require.NoError(t, first.Start(t.Context(), storageHost{client: client}))
			require.NoError(t, first.ConsumeMetrics(t.Context(), makeInput(false, 1, 2, 10)))
			require.NoError(t, first.ConsumeMetrics(t.Context(), makeInput(true, 1, 2, 100)))
			require.NoError(t, first.Shutdown(t.Context()))

			second := storageProcessor(t)
			require.NoError(t, second.Start(t.Context(), storageHost{client: client}))
			require.NoError(t, second.ConsumeMetrics(t.Context(), makeInput(false, 2, 3, 1)))
			require.NoError(t, second.ConsumeMetrics(t.Context(), makeInput(true, 2, 3, 2)))
			require.NoError(t, second.Shutdown(t.Context()))
			output := second.next.(*consumertest.MetricsSink).AllMetrics()
			require.Len(t, output, 2)
			for i, want := range []int64{11, 102} {
				dp := output[i].ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0)
				require.Equal(t, want, dp.IntValue())
				require.Equal(t, pcommon.Timestamp(1), dp.StartTimestamp())
			}
		})
	}
}

type indexWriteFailureClient struct{ *memClient }

func (c *indexWriteFailureClient) Set(ctx context.Context, key string, value []byte) error {
	if key == staleIndexKey {
		return errors.New("index write failed")
	}
	return c.memClient.Set(ctx, key, value)
}

type partialStateWriteClient struct{ *memClient }

func (c *partialStateWriteClient) Batch(ctx context.Context, ops ...*storage.Operation) error {
	if len(ops) > 0 {
		if err := c.memClient.Batch(ctx, ops[0]); err != nil {
			return err
		}
	}
	return errors.New("partial state write")
}

func TestStorageFailedFlushLeavesNoUntrackedState(t *testing.T) {
	for _, tt := range []struct {
		name       string
		client     func(*memClient) storage.Client
		wantStored int
	}{
		{"index failure", func(c *memClient) storage.Client { return &indexWriteFailureClient{c} }, 0},
		{"partial state write", func(c *memClient) storage.Client { return &partialStateWriteClient{c} }, 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				client := newMemClient()
				first, err := newPersistedState(t.Context(), tt.client(client), maps.Limit(10), time.Minute)
				require.NoError(t, err)
				keys := []streamID{{fingerprint: [32]byte{1}}, {fingerprint: [32]byte{2}}}
				for _, key := range keys {
					_, _, err = first.nums.LoadOrStore(t.Context(), key, guard(pmetric.NewNumberDataPoint()))
					require.NoError(t, err)
				}
				require.Error(t, first.flush(t.Context()))
				index := readStaleIndex(t, client)
				stored := 0
				for _, key := range keys {
					data, getErr := client.Get(t.Context(), "nums/"+streamHashKey(key))
					require.NoError(t, getErr)
					if len(data) > 0 {
						stored++
						require.Contains(t, index, streamHashKey(key))
					}
				}
				require.Equal(t, tt.wantStored, stored)
				// Restart before retrying the failed flush, then expire the old streams.
				second, err := newPersistedState(t.Context(), client, maps.Limit(10), time.Minute)
				require.NoError(t, err)
				time.Sleep(2 * time.Minute)
				require.NoError(t, second.flush(t.Context()))
				for _, key := range keys {
					data, getErr := client.Get(t.Context(), "nums/"+streamHashKey(key))
					require.NoError(t, getErr)
					require.Empty(t, data)
				}
				require.Empty(t, readStaleIndex(t, client))
			})
		})
	}
}

type blockingReadClient struct {
	*memClient
	entered chan struct{}
	release chan struct{}
}

func (c *blockingReadClient) Get(ctx context.Context, key string) ([]byte, error) {
	if key == staleIndexKey {
		return c.memClient.Get(ctx, key)
	}
	close(c.entered)
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.release:
		return nil, nil
	}
}

func TestStorageReadCancellation(t *testing.T) {
	for _, name := range []string{"request", "processor shutdown"} {
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				client := &blockingReadClient{memClient: newMemClient(), entered: make(chan struct{}), release: make(chan struct{})}
				p := storageProcessor(t)
				require.NoError(t, p.Start(t.Context(), storageHost{client: client}))
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				done := make(chan error, 1)
				go func() {
					done <- p.ConsumeMetrics(ctx, deltaMetrics(pmetric.MetricTypeSum, 1, 2, 1))
				}()
				<-client.entered
				if name == "request" {
					cancel()
				} else {
					p.cancel()
				}
				synctest.Wait()
				select {
				case err := <-done:
					require.ErrorIs(t, err, context.Canceled)
				default:
					t.Error("storage read did not stop after cancellation")
					close(client.release)
					<-done
				}
				require.NoError(t, p.Shutdown(t.Context()))
				require.Zero(t, p.last.Size())
			})
		})
	}
}

func TestStorageLimitIsNotStorageFailure(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.StorageID = new(component.MustNewID("file_storage"))
	cfg.MaxStreams = 0
	sink := &consumertest.MetricsSink{}
	proc, telemetry := setup(t, cfg, sink)
	require.NoError(t, proc.Start(t.Context(), storageHost{client: newMemClient()}))
	defer func() { require.NoError(t, proc.Shutdown(t.Context())) }()
	for _, typ := range []pmetric.MetricType{pmetric.MetricTypeSum, pmetric.MetricTypeHistogram, pmetric.MetricTypeExponentialHistogram} {
		require.NoError(t, proc.ConsumeMetrics(t.Context(), deltaMetrics(typ, 1, 2, 1)))
	}
	require.Empty(t, sink.AllMetrics())
	var rm metricdata.ResourceMetrics
	require.NoError(t, telemetry.reader.Collect(t.Context(), &rm))
	for _, scope := range rm.ScopeMetrics {
		for _, metric := range scope.Metrics {
			if metric.Name != "otelcol_deltatocumulative_datapoints" {
				continue
			}
			sum, ok := metric.Data.(metricdata.Sum[int64])
			require.True(t, ok)
			require.Len(t, sum.DataPoints, 1)
			require.Equal(t, int64(3), sum.DataPoints[0].Value)
			reason, ok := sum.DataPoints[0].Attributes.Value("error")
			require.True(t, ok)
			require.Equal(t, "limit", reason.AsString())
			return
		}
	}
	t.Fatal("datapoint metric missing")
}
