package resourceuuidprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceuuidprocessor"

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

// cacheEntry carries its own expiry: the expirable LRU of golang-lru starts a goroutine that cannot be stopped.
type cacheEntry struct {
	uuid    string
	expires time.Time
}

type resourceUUIDProcessor struct {
	logger *zap.Logger
	cfg    *Config
	client *podClient
	cache  *lru.Cache[string, cacheEntry]

	// missed is set when a record referred to a pod that is not cached, so the retry tick knows a poll is worth it.
	missed atomic.Bool

	// failures counts consecutive failed polls; only touched by refresh, which never runs concurrently.
	failures int

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func newResourceUUIDProcessor(logger *zap.Logger, cfg *Config) *resourceUUIDProcessor {
	// New only fails for a non-positive size, which Validate rejects.
	cache, _ := lru.New[string, cacheEntry](cfg.CacheSize)
	return &resourceUUIDProcessor{
		logger: logger,
		cfg:    cfg,
		client: newPodClient(cfg.Endpoint, cfg.Timeout),
		cache:  cache,
	}
}

func (p *resourceUUIDProcessor) lookup(uid string) (string, bool) {
	e, ok := p.cache.Get(uid)
	if !ok {
		return "", false
	}
	if time.Now().After(e.expires) {
		p.cache.Remove(uid)
		return "", false
	}
	return e.uuid, true
}

func (p *resourceUUIDProcessor) start(_ context.Context, _ component.Host) error {
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel

	p.logger.Info("resource uuid processor started",
		zap.String("endpoint", p.cfg.Endpoint),
		zap.Duration("refresh_interval", p.cfg.RefreshInterval),
		zap.Duration("retry_interval", p.cfg.RetryInterval),
		zap.Duration("cache_ttl", p.cfg.CacheTTL),
		zap.Int("cache_size", p.cfg.CacheSize))

	p.wg.Add(1)
	go p.loop(ctx)
	return nil
}

func (p *resourceUUIDProcessor) shutdown(context.Context) error {
	if p.cancel != nil {
		p.cancel()
	}
	p.wg.Wait()
	p.logger.Info("resource uuid processor stopped", zap.Int("cached_pods", p.cache.Len()))
	return nil
}

func (p *resourceUUIDProcessor) loop(ctx context.Context) {
	defer p.wg.Done()

	p.refresh(ctx)

	refresh := time.NewTicker(p.cfg.RefreshInterval)
	retry := time.NewTicker(p.cfg.RetryInterval)
	defer refresh.Stop()
	defer retry.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-refresh.C:
			p.missed.Store(false)
			p.refresh(ctx)
		case <-retry.C:
			if p.missed.Swap(false) {
				p.refresh(ctx)
			}
		}
	}
}

// failuresBeforeError is how many consecutive failed polls are logged as warnings before they become errors.
const failuresBeforeError = 3

// refresh pulls the pod list and caches every pod that has a uuid. On failure the cache is left as it is.
func (p *resourceUUIDProcessor) refresh(ctx context.Context) {
	pods, unresolved, err := p.client.fetch(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		p.failures++
		fields := []zap.Field{
			zap.String("endpoint", p.cfg.Endpoint),
			zap.Int("consecutive_failures", p.failures),
			zap.Int("cached_pods", p.cache.Len()),
			zap.Error(err),
		}
		if p.failures >= failuresBeforeError {
			p.logger.Error("pod uuid endpoint keeps failing; new pods will not get k8s.pod.uuid and cached ones expire after cache_ttl", fields...)
		} else {
			p.logger.Warn("failed to fetch pod uuids, keeping the cached ones", fields...)
		}
		return
	}

	if p.failures > 0 {
		p.logger.Info("pod uuid endpoint recovered", zap.Int("after_failures", p.failures))
		p.failures = 0
	}

	added := 0
	expires := time.Now().Add(p.cfg.CacheTTL)
	for uid, uuid := range pods {
		if _, ok := p.lookup(uid); !ok {
			added++
		}
		p.cache.Add(uid, cacheEntry{uuid: uuid, expires: expires})
	}
	p.logger.Info("refreshed pod uuids",
		zap.Int("pods_with_uuid", len(pods)),
		zap.Int("pods_without_uuid", unresolved),
		zap.Int("newly_cached", added),
		zap.Int("cached_pods", p.cache.Len()))
}

func (p *resourceUUIDProcessor) processLogs(_ context.Context, ld plog.Logs) (plog.Logs, error) {
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		attrs := rls.At(i).Resource().Attributes()
		uidVal, ok := attrs.Get(p.cfg.PodUIDAttribute)
		if !ok || uidVal.Str() == "" {
			continue
		}
		if uuid, found := p.lookup(uidVal.Str()); found {
			attrs.PutStr(p.cfg.TargetAttribute, uuid)
		} else {
			p.logger.Debug("no uuid cached for pod yet", zap.String("pod_uid", uidVal.Str()))
			p.missed.Store(true)
		}
	}
	return ld, nil
}
