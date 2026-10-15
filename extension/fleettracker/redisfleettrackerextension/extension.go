// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redisfleettrackerextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/fleettracker/redisfleettrackerextension"

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/fleettracker/redisfleettrackerextension/internal/metadata"
)

// unknownCount is the sentinel stored in count before any heartbeat has
// succeeded.
const unknownCount = -1

// notDeliveredYet is the sentinel lastDelivered value for a subscriber that
// has not received its first delivery.
const notDeliveredYet = -2

type fleetTrackerExtension struct {
	cfg       *Config
	logger    *zap.Logger
	telemetry *metadata.TelemetryBuilder
	memberID  string

	client *redis.Client
	key    string

	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu      sync.Mutex
	count   int
	subs    map[int]*subscriber
	nextID  int
	stopped bool

	wake chan struct{}
}

type subscriber struct {
	callback      func(count int)
	lastDelivered int
}

func computeMemberID(set extension.Settings) string {
	idSource := ""
	if v, ok := set.Resource.Attributes().Get("service.instance.id"); ok {
		idSource = v.AsString()
	}
	if idSource == "" {
		idSource = uuid.NewString()
	}
	sum := sha256.Sum256([]byte(idSource))
	return hex.EncodeToString(sum[:])[:16]
}

func newExtension(cfg *Config, logger *zap.Logger, telemetry *metadata.TelemetryBuilder, memberID string) *fleetTrackerExtension {
	return &fleetTrackerExtension{
		cfg:       cfg,
		logger:    logger,
		telemetry: telemetry,
		memberID:  memberID,
		key:       fmt.Sprintf("%s:%s", cfg.KeyPrefix, cfg.Fleet),
		count:     unknownCount,
		subs:      make(map[int]*subscriber),
		wake:      make(chan struct{}, 1),
	}
}

// Start builds the Redis client, attempts one synchronous heartbeat and
// observe, then launches the background loops regardless of whether that
// first attempt succeeded. A Redis outage at startup therefore does not
// prevent the collector from starting; consumers just see no member count
// until Redis becomes reachable.
func (e *fleetTrackerExtension) Start(ctx context.Context, _ component.Host) error {
	opts := &redis.Options{
		Addr:     e.cfg.Endpoint,
		Username: e.cfg.Username,
		Password: string(e.cfg.Password),
		DB:       e.cfg.DB,
	}
	if !e.cfg.TLS.Insecure {
		tlsCfg, err := e.cfg.TLS.LoadTLSConfig(ctx)
		if err != nil {
			return fmt.Errorf("redis_fleet_tracker: load tls config: %w", err)
		}
		opts.TLSConfig = tlsCfg
	}
	e.client = redis.NewClient(opts)

	startCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	if err := e.heartbeatAndObserve(startCtx); err != nil {
		e.logger.Warn("redis_fleet_tracker: initial heartbeat failed, will keep retrying in the background", zap.Error(err))
		e.telemetry.ExtensionRedisFleetTrackerHeartbeatFailures.Add(ctx, 1)
	}
	cancel()

	loopCtx, loopCancel := context.WithCancel(context.Background())
	e.cancel = loopCancel

	e.wg.Add(2)
	go e.heartbeatLoop(loopCtx)
	go e.notifyLoop(loopCtx)

	return nil
}

// Shutdown best-effort deregisters this member, stops the background loops,
// and closes the Redis client.
func (e *fleetTrackerExtension) Shutdown(ctx context.Context) error {
	e.mu.Lock()
	e.stopped = true
	e.mu.Unlock()

	if e.client != nil {
		deregisterCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		if err := e.client.ZRem(deregisterCtx, e.key, e.memberID).Err(); err != nil {
			e.logger.Warn("redis_fleet_tracker: deregister on shutdown failed", zap.Error(err))
		}
		cancel()
	}

	if e.cancel != nil {
		e.cancel()
	}
	e.wg.Wait()

	if e.client != nil {
		return e.client.Close()
	}
	return nil
}

// heartbeatAndObserve runs the register/prune/count/expire pipeline once and
// stores the resulting live member count.
func (e *fleetTrackerExtension) heartbeatAndObserve(ctx context.Context) error {
	nowMs := time.Now().UnixMilli()
	ttlMs := e.cfg.TTL.Milliseconds()

	pipe := e.client.Pipeline()
	pipe.ZAdd(ctx, e.key, redis.Z{Score: float64(nowMs + ttlMs), Member: e.memberID})
	pipe.ZRemRangeByScore(ctx, e.key, "-inf", fmt.Sprintf("%d", nowMs))
	card := pipe.ZCard(ctx, e.key)
	pipe.PExpire(ctx, e.key, 2*e.cfg.TTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("redis_fleet_tracker: heartbeat: %w", err)
	}

	count := int(card.Val())
	e.mu.Lock()
	e.count = count
	e.mu.Unlock()
	e.telemetry.ExtensionRedisFleetTrackerMemberCount.Record(ctx, int64(count))
	e.wakeNotifier()
	return nil
}

// heartbeatLoop runs heartbeatAndObserve on a jittered ticker until ctx is
// cancelled.
func (e *fleetTrackerExtension) heartbeatLoop(ctx context.Context) {
	defer e.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(jitter(e.cfg.HeartbeatInterval)):
		}

		tickCtx, cancel := context.WithTimeout(ctx, e.cfg.HeartbeatInterval)
		err := e.heartbeatAndObserve(tickCtx)
		cancel()
		if err != nil {
			e.logger.Warn("redis_fleet_tracker: heartbeat failed", zap.Error(err))
			e.telemetry.ExtensionRedisFleetTrackerHeartbeatFailures.Add(ctx, 1)
		}
	}
}

// jitter returns d scaled by a random factor within +/-10%.
func jitter(d time.Duration) time.Duration {
	factor := 0.9 + rand.Float64()*0.2
	return time.Duration(float64(d) * factor)
}

func (e *fleetTrackerExtension) wakeNotifier() {
	select {
	case e.wake <- struct{}{}:
	default:
	}
}

// notifyLoop delivers count updates to subscribers whose last-delivered
// count is stale, one at a time and outside the extension's mutex.
func (e *fleetTrackerExtension) notifyLoop(ctx context.Context) {
	defer e.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.wake:
		}

		for {
			cb, id, count, ok := e.nextDelivery()
			if !ok {
				break
			}
			cb(count)
			e.markDelivered(id, count)
		}
	}
}

// nextDelivery returns the next pending delivery, if any, under the lock.
func (e *fleetTrackerExtension) nextDelivery() (func(count int), int, int, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.count == unknownCount {
		return nil, 0, 0, false
	}
	for id, sub := range e.subs {
		if sub.lastDelivered != e.count {
			return sub.callback, id, e.count, true
		}
	}
	return nil, 0, 0, false
}

func (e *fleetTrackerExtension) markDelivered(id, count int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if sub, ok := e.subs[id]; ok {
		sub.lastDelivered = count
	}
}

// SubscribeMemberCount registers callback to be invoked with the live member
// count whenever it changes. The first delivery happens as soon as a count
// is known. cancel is idempotent; one in-flight delivery may still complete
// after cancel returns.
func (e *fleetTrackerExtension) SubscribeMemberCount(callback func(count int)) (cancel func(), err error) {
	e.mu.Lock()
	if e.stopped {
		e.mu.Unlock()
		return nil, errors.New("redis_fleet_tracker: extension is shut down")
	}
	id := e.nextID
	e.nextID++
	e.subs[id] = &subscriber{callback: callback, lastDelivered: notDeliveredYet}
	e.mu.Unlock()

	e.wakeNotifier()

	return func() {
		e.mu.Lock()
		delete(e.subs, id)
		e.mu.Unlock()
	}, nil
}
