// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsecsattributesprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/awsecsattributesprocessor"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/docker/docker/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/awsecsattributesprocessor/internal/metadata"
)

// metadataHTTPTimeout bounds calls to the per-container metadata endpoint.
const metadataHTTPTimeout = 15 * time.Second

// ecsCore holds the state and behavior shared by all four signal processors:
// the metadata cache, the Docker/ECS sync machinery, and the resource enrichment
// logic. Each signal processor embeds a *ecsCore and adds its typed Consume method.
type ecsCore struct {
	sync.RWMutex

	cfg       *Config
	logger    *zap.Logger
	endpoints endpointsFn

	// newDockerClient creates the Docker client used for container discovery and
	// event subscription. It is a field so tests can substitute a fake.
	newDockerClient func() (*client.Client, error)

	// metadata is the container-ID keyed metadata cache.
	metadata map[string]containerMetadata
	// metadataAge tracks when the cache was last evicted of stale entries.
	metadataAge time.Time

	// dockerClient is retained only when the daemon answered Ping, so it can be
	// closed on shutdown.
	dockerClient *client.Client

	stop chan struct{}
}

func newCore(logger *zap.Logger, cfg *Config, endpoints endpointsFn) *ecsCore {
	return &ecsCore{
		cfg:       cfg,
		logger:    logger,
		endpoints: endpoints,
		newDockerClient: func() (*client.Client, error) {
			return client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		},
		metadata:    make(map[string]containerMetadata),
		metadataAge: time.Now(),
		stop:        make(chan struct{}),
	}
}

// Capabilities reports that the processor mutates resource attributes.
func (*ecsCore) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

// Start initializes the Docker client (when reachable), performs an initial
// metadata sync, and launches the background sync goroutine.
func (e *ecsCore) Start(ctx context.Context, _ component.Host) error {
	var eventsClient *client.Client
	if cli, err := e.newDockerClient(); err != nil {
		if !isDockerClientCreationRecoverable(err) {
			return fmt.Errorf("failed to initialize Docker API client: %w", err)
		}
		e.logger.Warn("failed to initialize Docker API client; using ECS task metadata when available", zap.Error(err))
	} else {
		pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		_, pingErr := cli.Ping(pingCtx)
		cancel()
		if pingErr != nil {
			e.logger.Debug("docker daemon unreachable; skipping Docker events", zap.Error(pingErr))
			_ = cli.Close()
		} else {
			e.dockerClient = cli
			eventsClient = cli
		}
	}

	if err := e.runsync(ctx); err != nil {
		e.logger.Error("failed to sync metadata", zap.Error(err))
		return err
	}

	startDockerMetadataSync(e.logger, eventsClient, e.stop, func() error {
		return e.runsync(context.Background())
	})
	return nil
}

// Shutdown stops the background sync goroutine and closes the Docker client.
func (e *ecsCore) Shutdown(context.Context) error {
	close(e.stop)
	if e.dockerClient != nil {
		_ = e.dockerClient.Close()
	}
	return nil
}

// get returns the cached metadata for key, triggering a sync if the key is absent.
func (e *ecsCore) get(ctx context.Context, key string) (containerMetadata, error) {
	e.RLock()
	val, ok := e.metadata[key]
	e.RUnlock()
	if ok {
		return val, nil
	}

	if err := e.runsync(ctx); err != nil {
		return containerMetadata{}, fmt.Errorf("failed to sync metadata: %w", err)
	}

	e.RLock()
	val, ok = e.metadata[key]
	e.RUnlock()
	if !ok {
		return val, fmt.Errorf("metadata not found for container id after sync: %s", key)
	}
	return val, nil
}

// runsync refreshes the metadata cache from the discovered endpoints. Transient
// ECS metadata outages preserve the last-known-good cache rather than erroring.
func (e *ecsCore) runsync(ctx context.Context) error {
	endpoints, preload, err := e.endpoints(ctx, e.logger)
	if err != nil {
		if errors.Is(err, errECSTaskMetadataUnavailable) {
			return nil
		}
		return fmt.Errorf("failed to fetch metadata endpoints: %w", err)
	}

	if len(preload) > 0 {
		e.Lock()
		mergeIntoMetadataCache(e.metadata, preload)
		e.Unlock()
	}

	if err := e.syncMetadata(ctx, endpoints); err != nil {
		return fmt.Errorf("failed to update metadata endpoints: %w", err)
	}

	e.logger.Debug("metadata sync complete",
		zap.String("processor", metadata.Type.String()),
		zap.Int("endpoint_count", len(endpoints)))
	return nil
}

// syncMetadata fetches metadata for any newly seen endpoints and evicts cache
// entries that are no longer present once the cache TTL has elapsed.
func (e *ecsCore) syncMetadata(ctx context.Context, endpoints map[string][]string) error {
	e.Lock()
	defer e.Unlock()

	for k, urls := range endpoints {
		if _, ok := e.metadata[k]; ok {
			continue // already cached
		}
		if len(urls) == 0 {
			continue // ECS task path: metadata supplied via preload
		}

		md, err := fetchMetadata(ctx, urls[0]) // use the first available endpoint
		if err != nil {
			e.logger.Error("failed to fetch container metadata", zap.String("container_id", k), zap.Error(err))
			continue
		}
		e.metadata[k] = md
	}

	// remove keys no longer present in the current endpoint view, once per TTL.
	if time.Since(e.metadataAge) > time.Duration(e.cfg.CacheTTL)*time.Second {
		e.logger.Debug("evicting stale metadata")
		for k := range e.metadata {
			if _, ok := endpoints[k]; !ok {
				delete(e.metadata, k)
			}
		}
		e.metadataAge = time.Now()
	}

	return nil
}

// enrichResource reads the container ID from the resource attributes, looks up the
// cached metadata, and writes the configured attributes onto the resource.
func (e *ecsCore) enrichResource(ctx context.Context, res pcommon.Resource) {
	containerID := containerIDFromAttrs(res.Attributes(), e.cfg.Sources...)
	md, err := e.get(ctx, containerID)
	if err != nil {
		e.logger.Debug("unable to retrieve metadata",
			zap.String("container.id", containerID),
			zap.String("processor", metadata.Type.String()),
			zap.Error(err))
		return
	}

	for k, v := range md.flat() {
		val := fmt.Sprintf("%v", v)
		if !e.cfg.allowAttr(k) || val == "" {
			continue
		}
		res.Attributes().PutStr(k, val)
	}
}

// fetchMetadata retrieves and decodes a container containerMetadata document from url.
func fetchMetadata(ctx context.Context, url string) (containerMetadata, error) {
	var md containerMetadata
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return md, err
	}

	hc := &http.Client{Timeout: metadataHTTPTimeout}
	resp, err := hc.Do(req)
	if err != nil {
		return md, err
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&md); err != nil {
		return md, err
	}
	return md, nil
}

// containerIDFromAttrs reads the container ID from the first present source
// attribute and normalizes it to the 64-character Docker ID when possible.
func containerIDFromAttrs(attrs pcommon.Map, sources ...string) string {
	var id string
	for _, s := range sources {
		if v, ok := attrs.Get(s); ok {
			id = v.AsString()
			break
		}
	}
	return idReg.FindString(id)
}
