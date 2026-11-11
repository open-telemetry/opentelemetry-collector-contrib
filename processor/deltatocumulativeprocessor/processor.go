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
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"

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
	next   consumer.Metrics
	cfg    Config
	id     component.ID
	logger *zap.Logger

	last state
	aggr data.Aggregator

	ctx    context.Context
	cancel context.CancelFunc
	bgDone chan struct{}

	stale *xsync.Map[streamID, time.Time]
	tel   telemetry.Metrics
}

func newProcessor(
	cfg *Config,
	componentID component.ID,
	logger *zap.Logger,
	tel telemetry.Metrics,
	next consumer.Metrics,
) *deltaToCumulativeProcessor {
	ctx, cancel := context.WithCancel(context.Background())

	proc := deltaToCumulativeProcessor{
		next:   next,
		cfg:    *cfg,
		id:     componentID,
		logger: logger,
		last:   newState(int64(cfg.MaxStreams)),
		aggr:   delta.Aggregator{Aggregator: new(data.Adder)},
		ctx:    ctx,
		cancel: cancel,

		stale: xsync.NewMap[streamID, time.Time](),
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
	if p.cfg.StorageID != nil {
		requestCtx, cancel := context.WithCancel(ctx)
		stop := context.AfterFunc(p.ctx, cancel)
		defer func() {
			stop()
			cancel()
		}()
		ctx = requestCtx
	}
	now := time.Now()

	const (
		keep = true
		drop = false
	)

	zero := newVals()
	var storageErr error

	metrics.Filter(md, func(m metrics.Metric) bool {
		if m.AggregationTemporality() != pmetric.AggregationTemporalityDelta {
			return keep
		}

		// aggregate the datapoints.
		// using filter here, as the pmetric.*DataPoint are reference types so
		// we can modify them using their "value".
		m.Filter(func(stream identity.Stream, dp any) bool {
			id := streamID{Stream: stream}
			if p.cfg.StorageID != nil {
				id.fingerprint = streamFingerprint(m, dp.(interface{ Attributes() pcommon.Map }).Attributes())
			}
			// count the processed datatype.
			// uses whatever value of attrs has at return-time
			var attrs telemetry.Attributes
			defer func() { p.tel.Datapoints().Inc(ctx, attrs...) }()

			var err error
			switch dp := dp.(type) {
			case pmetric.NumberDataPoint:
				last, loaded, loadErr := p.last.nums.LoadOrStore(ctx, id, zero.nums)
				if errors.Is(loadErr, maps.ErrLimitExceeded) {
					// state is full, reject stream
					attrs.Set(telemetry.Error("limit"))
					return drop
				}
				if loadErr != nil {
					storageErr = loadErr
					attrs.Set(telemetry.Error("storage"))
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
			case pmetric.HistogramDataPoint:
				last, loaded, loadErr := p.last.hist.LoadOrStore(ctx, id, zero.hist)
				if errors.Is(loadErr, maps.ErrLimitExceeded) {
					// state is full, reject stream
					attrs.Set(telemetry.Error("limit"))
					return drop
				}
				if loadErr != nil {
					storageErr = loadErr
					attrs.Set(telemetry.Error("storage"))
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
			case pmetric.ExponentialHistogramDataPoint:
				last, loaded, loadErr := p.last.expo.LoadOrStore(ctx, id, zero.expo)
				if errors.Is(loadErr, maps.ErrLimitExceeded) {
					// state is full, reject stream
					attrs.Set(telemetry.Error("limit"))
					return drop
				}
				if loadErr != nil {
					storageErr = loadErr
					attrs.Set(telemetry.Error("storage"))
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

	if storageErr != nil {
		p.logger.Error("Dropped data points because cumulative state could not be loaded", zap.Error(storageErr))
	}
	if md.MetricCount() > 0 {
		if err := p.next.ConsumeMetrics(ctx, md); err != nil {
			return err
		}
	}
	// A retry would reprocess points already accumulated and forwarded.
	if storageErr != nil {
		return consumererror.NewPermanent(storageErr)
	}
	return nil
}

func (p *deltaToCumulativeProcessor) Start(ctx context.Context, host component.Host) error {
	var client storage.Client
	if p.cfg.StorageID != nil {
		var err error
		client, err = toStorageClient(ctx, *p.cfg.StorageID, host, p.id, pipeline.SignalMetrics)
		if err != nil {
			return fmt.Errorf("getting storage client: %w", err)
		}
	}
	if client != nil {
		var err error
		p.last, err = newPersistedState(ctx, client, p.last.ctx, p.cfg.MaxStale)
		if err != nil {
			return errors.Join(err, client.Close(ctx))
		}
	}
	p.bgDone = make(chan struct{})

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
					p.stale.Range(func(id streamID, last time.Time) bool {
						if now.Sub(last) > p.cfg.MaxStale {
							// ConsumeMetrics may have refreshed the timestamp during iteration.
							if cur, ok := p.stale.Load(id); ok && now.Sub(cur) > p.cfg.MaxStale {
								_, _, numErr := p.last.nums.LoadAndDelete(p.ctx, id)
								_, _, histErr := p.last.hist.LoadAndDelete(p.ctx, id)
								_, _, expoErr := p.last.expo.LoadAndDelete(p.ctx, id)
								if err := errors.Join(numErr, histErr, expoErr); err != nil {
									p.logger.Error("Failed to delete stale cumulative state", zap.Error(err))
								} else {
									p.stale.Delete(id)
								}
							}
						}
						return true
					})
				}

				// Periodic I/O is cancelled on shutdown; final flush uses the shutdown context.
				if p.last.flush != nil {
					if err := p.last.flush(p.ctx); err != nil && p.ctx.Err() == nil {
						p.logger.Error("Failed to persist cumulative state", zap.Error(err))
					}
				}
			}
		}
	}()

	return nil
}

func (p *deltaToCumulativeProcessor) Shutdown(ctx context.Context) error {
	p.cancel()
	if p.bgDone != nil {
		select {
		case <-p.bgDone:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	var err error
	if p.last.flush != nil {
		err = p.last.flush(ctx)
	}
	if p.last.close != nil {
		err = errors.Join(err, p.last.close(ctx))
	}
	return err
}

func (*deltaToCumulativeProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

type Storage[K comparable, V any] interface {
	LoadOrStore(ctx context.Context, key K, value V) (actual V, loaded bool, err error)
	LoadAndDelete(ctx context.Context, key K) (value V, loaded bool, err error)
}

// state keeps a cumulative value, aggregated over time, per stream
type state struct {
	ctx   maps.Context
	nums  Storage[streamID, *mutex[pmetric.NumberDataPoint]]
	hist  Storage[streamID, *mutex[pmetric.HistogramDataPoint]]
	expo  Storage[streamID, *mutex[pmetric.ExponentialHistogramDataPoint]]
	flush func(ctx context.Context) error
	close func(ctx context.Context) error
}

func newState(maxStreams int64) state {
	limit := maps.Limit(maxStreams)
	return state{
		ctx:  limit,
		nums: maps.New[streamID, *mutex[pmetric.NumberDataPoint]](limit),
		hist: maps.New[streamID, *mutex[pmetric.HistogramDataPoint]](limit),
		expo: maps.New[streamID, *mutex[pmetric.ExponentialHistogramDataPoint]](limit),
	}
}

func (s state) Size() int {
	return int(s.ctx.Size())
}

const staleIndexKey = "_index/stale_keys"

func newPersistedState(ctx context.Context, client storage.Client, limit maps.Context, maxStale time.Duration) (state, error) {
	nums := maps.NewPersisted(client, "nums", limit, streamHashKey, marshalNumberDP, unmarshalNumberDP)
	hist := maps.NewPersisted(client, "hist", limit, streamHashKey, marshalHistogramDP, unmarshalHistogramDP)
	expo := maps.NewPersisted(client, "expo", limit, streamHashKey, marshalExpoDP, unmarshalExpoDP)

	// Retain previous keys until deletion succeeds, including across short restarts.
	pending, err := loadStaleIndex(ctx, client)
	if err != nil {
		return state{}, fmt.Errorf("load stale index: %w", err)
	}
	cleanupAt := time.Now().Add(maxStale)
	var flushMu sync.Mutex

	return state{
		ctx:   limit,
		close: client.Close,
		nums:  nums,
		hist:  hist,
		expo:  expo,
		flush: func(ctx context.Context) error {
			flushMu.Lock()
			defer flushMu.Unlock()
			var cleanupErr error
			if maxStale > 0 && !time.Now().Before(cleanupAt) {
				cleanupErr = cleanupOrphans(ctx, pending, nums, hist, expo)
			}
			numsKeys, histKeys, expoKeys := nums.Keys(), hist.Keys(), expo.Keys()
			remaining := make([]string, 0, len(pending))
			for key := range pending {
				remaining = append(remaining, key)
			}
			// Register keys first so even a partial write remains discoverable after restart.
			if err := persistStaleIndex(ctx, client, numsKeys, histKeys, expoKeys, remaining); err != nil {
				return errors.Join(cleanupErr, err)
			}
			return errors.Join(cleanupErr, nums.Flush(ctx, numsKeys), hist.Flush(ctx, histKeys), expo.Flush(ctx, expoKeys))
		},
	}, nil
}

func persistStaleIndex(ctx context.Context, client storage.Client, keysets ...[]string) error {
	var size int
	for _, keys := range keysets {
		for _, k := range keys {
			size += len(k) + 1
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

func loadStaleIndex(ctx context.Context, client storage.Client) (map[string]struct{}, error) {
	data, err := client.Get(ctx, staleIndexKey)
	if err != nil {
		return nil, err
	}
	keys := make(map[string]struct{})
	for line := range bytes.SplitSeq(data, []byte("\n")) {
		if k := string(line); k != "" {
			keys[k] = struct{}{}
		}
	}
	return keys, nil
}

type deletableByHash interface {
	Keys() []string
	DeleteByHashKey(ctx context.Context, hashKey string) error
}

// Failed deletes remain pending so cleanup can retry after a restart.
func cleanupOrphans(ctx context.Context, pending map[string]struct{}, persisted ...deletableByHash) error {
	for _, p := range persisted {
		for _, key := range p.Keys() {
			delete(pending, key)
		}
	}
	var errs []error
	for key := range pending {
		var err error
		for _, p := range persisted {
			err = errors.Join(err, p.DeleteByHashKey(ctx, key))
		}
		if err != nil {
			errs = append(errs, err)
		} else {
			delete(pending, key)
		}
	}
	return errors.Join(errs...)
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
	if host == nil {
		return nil, errNoStorageClient
	}
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
