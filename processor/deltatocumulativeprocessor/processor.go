// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package deltatocumulativeprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor"

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/puzpuzpuz/xsync/v4"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/processor"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/delta"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/maps"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/telemetry"
)

var _ processor.Metrics = (*deltaToCumulativeProcessor)(nil)

var (
	errNoStorageClient    = errors.New("storage client not found")
	errWrongExtensionType = errors.New("extension is not a storage extension")
)

type deltaToCumulativeProcessor struct {
	next consumer.Metrics
	cfg  Config
	id   component.ID

	last state
	aggr data.Aggregator

	ctx    context.Context
	cancel context.CancelFunc
	bgDone chan struct{} // closed when background goroutine exits

	stale *xsync.Map[identity.Stream, time.Time]
	tel   telemetry.Metrics
}

func newProcessor(
	cfg *Config,
	componentID component.ID,
	tel telemetry.Metrics,
	next consumer.Metrics,
) *deltaToCumulativeProcessor {
	ctx, cancel := context.WithCancel(context.Background())

	proc := deltaToCumulativeProcessor{
		next:   next,
		cfg:    *cfg,
		id:     componentID,
		last:   newState(ctx, int64(cfg.MaxStreams)),
		aggr:   delta.Aggregator{Aggregator: new(data.Adder)},
		ctx:    ctx,
		cancel: cancel,

		stale: xsync.NewMap[identity.Stream, time.Time](),
		tel:   tel,
	}

	tel.WithTracked(proc.last.Size)
	cfg.Metrics(tel)

	return &proc
}

type vals struct {
	nums *mutex[pmetric.NumberDataPoint]
	hist *mutex[pmetric.HistogramDataPoint]
	expo *mutex[pmetric.ExponentialHistogramDataPoint]
}

func newVals() vals {
	return vals{
		nums: guard(pmetric.NewNumberDataPoint()),
		hist: guard(pmetric.NewHistogramDataPoint()),
		expo: guard(pmetric.NewExponentialHistogramDataPoint()),
	}
}

func (p *deltaToCumulativeProcessor) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	now := time.Now()

	const (
		keep = true
		drop = false
	)

	zero := newVals()

	metrics.Filter(md, func(m metrics.Metric) bool {
		if m.AggregationTemporality() != pmetric.AggregationTemporalityDelta {
			return keep
		}

		// aggregate the datapoints.
		// using filter here, as the pmetric.*DataPoint are reference types so
		// we can modify them using their "value".
		m.Filter(func(id identity.Stream, dp any) bool {
			// count the processed datatype.
			// uses whatever value of attrs has at return-time
			var attrs telemetry.Attributes
			defer func() { p.tel.Datapoints().Inc(ctx, attrs...) }()

			var err error
			switch dp := dp.(type) {
			case pmetric.NumberDataPoint:
				last, loaded := p.last.nums.LoadOrStore(id, zero.nums)
				if maps.Exceeded(last, loaded) {
					// state is full, reject stream
					attrs.Set(telemetry.Error("limit"))
					return drop
				}

				// stream is ok and active, update stale tracker
				p.stale.Store(id, now)

				if !loaded {
					// cached zero was stored, alloc new one
					zero.nums = guard(pmetric.NewNumberDataPoint())
				}

				last.use(func(last pmetric.NumberDataPoint) {
					err = p.aggr.Numbers(last, dp)
					last.CopyTo(dp)
				})
				p.last.nums.Store(id, last)
			case pmetric.HistogramDataPoint:
				last, loaded := p.last.hist.LoadOrStore(id, zero.hist)
				if maps.Exceeded(last, loaded) {
					// state is full, reject stream
					attrs.Set(telemetry.Error("limit"))
					return drop
				}

				// stream is ok and active, update stale tracker
				p.stale.Store(id, now)

				if !loaded {
					// cached zero was stored, alloc new one
					zero.hist = guard(pmetric.NewHistogramDataPoint())
				}

				last.use(func(last pmetric.HistogramDataPoint) {
					err = p.aggr.Histograms(last, dp)
					last.CopyTo(dp)
				})
				p.last.hist.Store(id, last)
			case pmetric.ExponentialHistogramDataPoint:
				last, loaded := p.last.expo.LoadOrStore(id, zero.expo)
				if maps.Exceeded(last, loaded) {
					// state is full, reject stream
					attrs.Set(telemetry.Error("limit"))
					return drop
				}

				// stream is ok and active, update stale tracker
				p.stale.Store(id, now)

				if !loaded {
					// cached zero was stored, alloc new one
					zero.expo = guard(pmetric.NewExponentialHistogramDataPoint())
				}

				last.use(func(last pmetric.ExponentialHistogramDataPoint) {
					err = p.aggr.Exponential(last, dp)
					last.CopyTo(dp)
				})
				p.last.expo.Store(id, last)
			}

			if err != nil {
				attrs.Set(telemetry.Cause(err))
				return drop
			}

			return keep
		})

		// all remaining datapoints of this metric are now cumulative
		m.Typed().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		// if no datapoints remain, drop empty metric
		return m.Typed().Len() > 0
	})

	// no need to continue pipeline if we dropped all metrics
	if md.MetricCount() == 0 {
		return nil
	}
	return p.next.ConsumeMetrics(ctx, md)
}

func (p *deltaToCumulativeProcessor) Start(ctx context.Context, host component.Host) error {
	// background goroutine: stale cleanup + lazy flush
	p.bgDone = make(chan struct{})

	if p.cfg.StorageID != nil && host != nil {
		client, err := toStorageClient(ctx, *p.cfg.StorageID, host, p.id, pipeline.SignalMetrics)
		if err != nil {
			return fmt.Errorf("getting storage client: %w", err)
		}
		switch {
		case p.cfg.WriteThrough:
			p.last = newWriteThroughState(client, int64(p.cfg.MaxStreams), p.cfg.MaxStale, p.bgDone)
		default:
			p.last = newPersistedState(client, int64(p.cfg.MaxStreams), p.cfg.MaxStale, p.bgDone)
		}
	}

	go func() {
		defer close(p.bgDone)
		tick := time.NewTicker(time.Minute)
		defer tick.Stop()
		for {
			select {
			case <-p.ctx.Done():
				return
			case <-tick.C:
				if p.cfg.MaxStale != 0 {
					now := time.Now()
					p.stale.Range(func(id identity.Stream, last time.Time) bool {
						if now.Sub(last) > p.cfg.MaxStale {
							// Re-check: ConsumeMetrics may have refreshed
							// the timestamp concurrently during iteration.
							if cur, ok := p.stale.Load(id); ok && now.Sub(cur) > p.cfg.MaxStale {
								p.last.nums.LoadAndDelete(id)
								p.last.hist.LoadAndDelete(id)
								p.last.expo.LoadAndDelete(id)
								p.stale.Delete(id)
							}
						}
						return true
					})
				}

				// lazy flush to persistent storage — intentionally uses
				// background context because the processor's ctx may be cancelled.
				if p.last.flush != nil {
					_ = p.last.flush(context.Background())
				}
			}
		}
	}()

	return nil
}

func (p *deltaToCumulativeProcessor) Shutdown(ctx context.Context) error {
	// stop background goroutine first and wait for it to exit,
	// ensuring no concurrent flush can happen
	p.cancel()
	if p.bgDone != nil {
		<-p.bgDone
	}

	// final flush after background goroutine is gone
	if p.last.flush != nil {
		return p.last.flush(ctx)
	}
	return nil
}

func (*deltaToCumulativeProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

type Storage[K comparable, V any] interface {
	LoadOrStore(key K, value V) (actual V, loaded bool)
	LoadAndDelete(key K) (value V, loaded bool)
	Store(key K, value V) // Persist after mutation. No-op for in-memory and write-back maps.
}

// persistedStorage adapts a [maps.Persisted] to the [Storage] interface by
// capturing a context.Context. The context should outlive storage operations
// and must NOT be the processor's cancellable lifecycle context.
type persistedStorage[K comparable, V any] struct {
	ctx context.Context
	m   *maps.Persisted[K, V]
}

func (ps *persistedStorage[K, V]) LoadOrStore(key K, value V) (V, bool) {
	return ps.m.LoadOrStore(ps.ctx, key, value)
}

func (ps *persistedStorage[K, V]) LoadAndDelete(key K) (V, bool) {
	return ps.m.LoadAndDelete(ps.ctx, key)
}

// Store is a no-op for write-back mode. Dirty tracking in LoadOrStore
// already ensures changed values are flushed.
func (*persistedStorage[K, V]) Store(_ K, _ V) {}

// writeThroughStorage adapts a [maps.WriteThrough] to the [Storage] interface
// by capturing a context.Context. Unlike [persistedStorage], every Store call
// writes directly to the backing storage.
type writeThroughStorage[K comparable, V any] struct {
	ctx context.Context
	m   *maps.WriteThrough[K, V]
}

func (ws *writeThroughStorage[K, V]) LoadOrStore(key K, value V) (V, bool) {
	return ws.m.LoadOrStore(ws.ctx, key, value)
}

func (ws *writeThroughStorage[K, V]) LoadAndDelete(key K) (V, bool) {
	return ws.m.LoadAndDelete(ws.ctx, key)
}

func (ws *writeThroughStorage[K, V]) Store(key K, value V) {
	_ = ws.m.Store(ws.ctx, key, value)
}

// state keeps a cumulative value, aggregated over time, per stream
type state struct {
	ctx   maps.Context
	nums  Storage[identity.Stream, *mutex[pmetric.NumberDataPoint]]
	hist  Storage[identity.Stream, *mutex[pmetric.HistogramDataPoint]]
	expo  Storage[identity.Stream, *mutex[pmetric.ExponentialHistogramDataPoint]]
	flush func(ctx context.Context) error // optional: called periodically and on shutdown to persist state
}

func newState(_ context.Context, maxStreams int64) state {
	limit := maps.Limit(maxStreams)
	return state{
		ctx:  limit,
		nums: maps.New[identity.Stream, *mutex[pmetric.NumberDataPoint]](limit),
		hist: maps.New[identity.Stream, *mutex[pmetric.HistogramDataPoint]](limit),
		expo: maps.New[identity.Stream, *mutex[pmetric.ExponentialHistogramDataPoint]](limit),
	}
}

func (s state) Size() int {
	return int(s.ctx.Size())
}

const staleIndexKey = "_index/stale_keys"

func newPersistedState(client storage.Client, maxStreams int64, maxStale time.Duration, done <-chan struct{}) state {
	limit := maps.Limit(maxStreams)

	// Use a background context for storage operations — must not be tied
	// to the processor's cancellable lifecycle context so that the final
	// shutdown flush can succeed.
	storageCtx := context.Background()

	nums := maps.NewPersisted(client, "nums", limit, streamHashKey, marshalNumberDP, unmarshalNumberDP)
	hist := maps.NewPersisted(client, "hist", limit, streamHashKey, marshalHistogramDP, unmarshalHistogramDP)
	expo := maps.NewPersisted(client, "expo", limit, streamHashKey, marshalExpoDP, unmarshalExpoDP)

	// Load the stale index from the previous session and schedule cleanup
	// for orphaned storage entries after MaxStale.
	scheduleOrphanCleanup(storageCtx, client, done, maxStale, nums, hist, expo)

	return state{
		ctx:  limit,
		nums: &persistedStorage[identity.Stream, *mutex[pmetric.NumberDataPoint]]{ctx: storageCtx, m: nums},
		hist: &persistedStorage[identity.Stream, *mutex[pmetric.HistogramDataPoint]]{ctx: storageCtx, m: hist},
		expo: &persistedStorage[identity.Stream, *mutex[pmetric.ExponentialHistogramDataPoint]]{ctx: storageCtx, m: expo},
		flush: func(ctx context.Context) error {
			numsR := nums.Flush(ctx)
			histR := hist.Flush(ctx)
			expoR := expo.Flush(ctx)
			flushErr := errors.Join(numsR.Err, histR.Err, expoR.Err)
			indexErr := persistStaleIndex(ctx, client, numsR.Keys, histR.Keys, expoR.Keys)
			return errors.Join(flushErr, indexErr)
		},
	}
}

func newWriteThroughState(client storage.Client, maxStreams int64, maxStale time.Duration, done <-chan struct{}) state {
	limit := maps.Limit(maxStreams)
	storageCtx := context.Background()

	nums := maps.NewWriteThrough(client, "nums", limit, streamHashKey, marshalNumberDP, unmarshalNumberDP)
	hist := maps.NewWriteThrough(client, "hist", limit, streamHashKey, marshalHistogramDP, unmarshalHistogramDP)
	expo := maps.NewWriteThrough(client, "expo", limit, streamHashKey, marshalExpoDP, unmarshalExpoDP)

	scheduleOrphanCleanup(storageCtx, client, done, maxStale, nums, hist, expo)

	return state{
		ctx:  limit,
		nums: &writeThroughStorage[identity.Stream, *mutex[pmetric.NumberDataPoint]]{ctx: storageCtx, m: nums},
		hist: &writeThroughStorage[identity.Stream, *mutex[pmetric.HistogramDataPoint]]{ctx: storageCtx, m: hist},
		expo: &writeThroughStorage[identity.Stream, *mutex[pmetric.ExponentialHistogramDataPoint]]{ctx: storageCtx, m: expo},
		flush: func(ctx context.Context) error {
			// Write-through: values are already persisted via Store.
			// Only persist the stale index for orphan cleanup.
			return persistStaleIndex(ctx, client, nums.Keys(), hist.Keys(), expo.Keys())
		},
	}
}

// persistStaleIndex saves the union of all active stream hash keys so that a
// future session can identify and clean up orphaned storage entries.
// The keysets come from different metric types (nums, hist, expo) and never
// overlap, so no deduplication is needed.
func persistStaleIndex(ctx context.Context, client storage.Client, keysets ...[]string) error {
	var size int
	for _, keys := range keysets {
		for _, k := range keys {
			size += len(k) + 1 // +1 for newline
		}
	}
	buf := make([]byte, 0, size)
	for _, keys := range keysets {
		for _, k := range keys {
			buf = append(buf, k...)
			buf = append(buf, '\n')
		}
	}
	return client.Set(ctx, staleIndexKey, buf)
}

// loadStaleIndex reads the previously persisted stream hash keys.
func loadStaleIndex(ctx context.Context, client storage.Client) map[string]struct{} {
	data, err := client.Get(ctx, staleIndexKey)
	if err != nil || len(data) == 0 {
		return nil
	}
	keys := make(map[string]struct{})
	for line := range bytes.SplitSeq(data, []byte("\n")) {
		if k := string(line); k != "" {
			keys[k] = struct{}{}
		}
	}
	return keys
}

type deletableByHash interface {
	Keys() []string
	DeleteByHashKey(ctx context.Context, hashKey string) error
}

// scheduleOrphanCleanup loads the stale index from the previous session and
// starts a one-shot goroutine that, after MaxStale elapses, deletes any
// storage entries that were NOT accessed (and thus not loaded into cache) since
// startup. This prevents orphaned streams from accumulating in storage across
// restarts.
func scheduleOrphanCleanup(ctx context.Context, client storage.Client, done <-chan struct{}, maxStale time.Duration, persisted ...deletableByHash) {
	prevKeys := loadStaleIndex(ctx, client)
	if len(prevKeys) == 0 || maxStale == 0 {
		return
	}

	go func() {
		timer := time.NewTimer(maxStale)
		defer timer.Stop()
		select {
		case <-done:
			return
		case <-timer.C:
		}

		// Collect currently active keys across all persisted maps
		active := make(map[string]struct{})
		for _, p := range persisted {
			for _, k := range p.Keys() {
				active[k] = struct{}{}
			}
		}

		// Delete orphans: keys from previous session that are not active now
		for k := range prevKeys {
			if _, ok := active[k]; ok {
				continue
			}
			for _, p := range persisted {
				_ = p.DeleteByHashKey(ctx, k)
			}
		}
	}()
}

type mutex[T any] struct {
	mtx sync.Mutex
	v   T
}

func (mtx *mutex[T]) use(do func(T)) {
	mtx.mtx.Lock()
	do(mtx.v)
	mtx.mtx.Unlock()
}

func guard[T any](v T) *mutex[T] {
	return &mutex[T]{v: v}
}

func toStorageClient(ctx context.Context, storageID component.ID, host component.Host, ownerID component.ID, signal pipeline.Signal) (storage.Client, error) {
	ext, found := host.GetExtensions()[storageID]
	if !found {
		return nil, errNoStorageClient
	}

	storageExt, ok := ext.(storage.Extension)
	if !ok {
		return nil, errWrongExtensionType
	}

	return storageExt.GetClient(ctx, component.KindProcessor, ownerID, signal.String())
}
