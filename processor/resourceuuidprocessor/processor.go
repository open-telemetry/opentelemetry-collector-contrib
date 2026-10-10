package resourceuuidprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceuuidprocessor"

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

// cacheEntry tracks when it was last used so that idle entries expire; the expirable LRU of golang-lru
// starts a goroutine that cannot be stopped.
type cacheEntry struct {
	resourceMetadata
	lastUsed atomic.Int64 // unix nanoseconds
}

type resourceUUIDProcessor struct {
	logger *zap.Logger
	cfg    *Config
	client *podClient
	cache  *lru.Cache[string, *cacheEntry]

	// missed is set when a record referred to a pod that is not cached, so the retry tick knows a poll is worth it.
	missed atomic.Bool

	// failures counts consecutive failed polls; only touched by refresh, which never runs concurrently.
	failures int

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func newResourceUUIDProcessor(logger *zap.Logger, cfg *Config) *resourceUUIDProcessor {
	// New only fails for a non-positive size, which Validate rejects.
	cache, _ := lru.New[string, *cacheEntry](cfg.CacheSize)
	return &resourceUUIDProcessor{
		logger: logger,
		cfg:    cfg,
		client: newPodClient(cfg.Endpoint, cfg.Timeout),
		cache:  cache,
	}
}

func (p *resourceUUIDProcessor) lookup(uid string) (resourceMetadata, bool) {
	e, ok := p.cache.Get(uid)
	if !ok {
		return resourceMetadata{}, false
	}
	now := time.Now()
	if now.Sub(time.Unix(0, e.lastUsed.Load())) > p.cfg.CacheTTL {
		p.cache.Remove(uid)
		return resourceMetadata{}, false
	}
	e.lastUsed.Store(now.UnixNano())
	return e.resourceMetadata, true
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

// refresh replaces known metadata from a successful snapshot, retaining the cache on failure.
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
			p.logger.Error("pod uuid endpoint keeps failing; new pods will not get k8s.pod.uuid and cached ones expire after sitting unused for cache_ttl", fields...)
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
	now := time.Now()
	// Entries leave the cache only after sitting unused for cache_ttl, not because a snapshot omits them.
	for _, uid := range p.cache.Keys() {
		if e, ok := p.cache.Peek(uid); ok && now.Sub(time.Unix(0, e.lastUsed.Load())) > p.cfg.CacheTTL {
			p.cache.Remove(uid)
		}
	}
	resolved := 0
	for uid, metadata := range pods {
		if strings.HasPrefix(uid, nodeKeyPrefix) != p.cfg.NodeLogs {
			continue
		}
		entry := &cacheEntry{resourceMetadata: metadata}
		if existing, ok := p.cache.Peek(uid); ok {
			entry.lastUsed.Store(existing.lastUsed.Load())
		} else {
			entry.lastUsed.Store(now.UnixNano())
			added++
		}
		p.cache.Add(uid, entry)
		if metadata.uuid != "" {
			resolved++
		}
	}
	p.logger.Info("refreshed pod uuids",
		zap.Int("resources_with_uuid", resolved),
		zap.Int("pods_without_uuid", unresolved),
		zap.Int("newly_cached", added),
		zap.Int("cached_pods", p.cache.Len()))
}

func (p *resourceUUIDProcessor) processLogs(_ context.Context, ld plog.Logs) (plog.Logs, error) {
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		attrs := rls.At(i).Resource().Attributes()
		uidVal, ok := attrs.Get(p.cfg.PodUIDAttribute)
		key := uidVal.Str()
		if p.cfg.NodeLogs {
			// Never fall back to node identity for a pod whose UUID has not resolved.
			if ok && key != "" {
				continue
			}
			uidVal, ok = attrs.Get("k8s.node.name")
			key = nodeKeyPrefix + uidVal.Str()
		}
		if !ok || uidVal.Str() == "" {
			continue
		}
		if metadata, found := p.lookup(key); found {
			if cluster, exists := attrs.Get("k8s.cluster.name"); exists && metadata.attributes["k8s.cluster.name"] != "" && cluster.Str() != metadata.attributes["k8s.cluster.name"] {
				p.logger.Warn("skipping metadata from a different cluster", zap.String("resource_key", key))
				continue
			}
			for name, value := range metadata.attributes {
				if value != "" {
					attrs.PutStr(name, value)
				}
			}
			if metadata.uuid != "" {
				attrs.PutStr("resourceUUID", metadata.uuid)
				if !p.cfg.NodeLogs {
					attrs.PutStr("k8s.pod.resourceUUID", metadata.uuid)
					attrs.PutStr(p.cfg.TargetAttribute, metadata.uuid)
				}
			} else {
				p.missed.Store(true)
			}
		} else {
			p.logger.Debug("no uuid cached for pod yet", zap.String("pod_uid", uidVal.Str()))
			p.missed.Store(true)
		}
	}
	return ld, nil
}
