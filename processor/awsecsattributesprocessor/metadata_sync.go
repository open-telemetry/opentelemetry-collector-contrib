// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsecsattributesprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/awsecsattributesprocessor"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/moby/moby/api/types/events"
	"github.com/moby/moby/client"
	"go.uber.org/zap"
)

const (
	ecsContainerMetadataURIv4 = "ECS_CONTAINER_METADATA_URI_V4"
	ecsContainerMetadataURI   = "ECS_CONTAINER_METADATA_URI"

	// syncInterval is how often the metadata cache is refreshed in the background.
	syncInterval = 60 * time.Second
)

var (
	// ecsMetadataReg matches a container environment variable that exposes an ECS
	// metadata endpoint, capturing the endpoint URL.
	ecsMetadataReg = regexp.MustCompile(fmt.Sprintf("(?:%s|%s)=(.*)", ecsContainerMetadataURI, ecsContainerMetadataURIv4))

	// idReg matches a full-length (64 hex char) Docker/container ID, used to
	// normalize IDs read from resource attributes (e.g. stripping a -json.log suffix).
	idReg = regexp.MustCompile(`[a-z0-9]{64}`)
)

// endpointsFn discovers the per-container ECS metadata endpoints on the host. It
// returns a map of cache-key -> endpoint URLs and, when the data is sourced from
// the ECS task metadata endpoint, a preloaded slice of container containerMetadata.
type endpointsFn func(ctx context.Context, logger *zap.Logger) (map[string][]string, []containerMetadata, error)

// ecsTaskMetadataV4Response is the JSON body from GET ${ECS_CONTAINER_METADATA_URI_V4}/task.
type ecsTaskMetadataV4Response struct {
	Containers []containerMetadata `json:"Containers"`
}

// logECSTaskMetadataModeOnce logs ECS task-metadata success at most once per
// process to avoid noisy periodic syncs.
var logECSTaskMetadataModeOnce sync.Once

// logECSTaskMetadataFallbackWarnOnce warns about ECS task-metadata being
// unavailable at most once per process.
var logECSTaskMetadataFallbackWarnOnce sync.Once

// errECSTaskMetadataUnavailable marks recoverable failures while querying ECS task
// metadata. Callers should skip the cache refresh (preserving last-known-good data)
// instead of treating this as a hard startup/configuration failure.
var errECSTaskMetadataUnavailable = errors.New("ecs task metadata unavailable")

// isDockerUnavailableError reports whether err indicates the Docker daemon is not
// reachable (socket missing, connection refused, named pipe absent, etc.).
func isDockerUnavailableError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	// Linux socket / permission / daemon down; Windows named pipe missing or access denied.
	substrs := []string{
		"connection refused",
		"no such file or directory",
		"cannot find the file",
		"the system cannot find the file specified",
		"docker_engine",
		"//./pipe/",
		"named pipe",
		"elevated privileges",
		"access is denied",
		"error while dialing",
	}
	for _, s := range substrs {
		if strings.Contains(msg, s) {
			return true
		}
	}
	return false
}

// isInvalidDockerClientConfigError matches NewClientWithOpts / FromEnv failures
// caused by bad DOCKER_HOST or similar user misconfiguration (not "daemon not running").
func isInvalidDockerClientConfigError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	blocklist := []string{
		"unable to parse",
		"error parsing",
		"could not parse",
		"invalid bind",
		"invalid proto",
		"invalid url",
		"invalid host",
		"malformed",
		"unsupported protocol",
	}
	for _, s := range blocklist {
		if strings.Contains(msg, s) {
			return true
		}
	}
	return false
}

// isDockerClientCreationRecoverable is true when NewClientWithOpts failed because
// the engine endpoint is missing or unreachable, so ECS task metadata is a
// reasonable fallback.
func isDockerClientCreationRecoverable(err error) bool {
	if err == nil || isInvalidDockerClientConfigError(err) {
		return false
	}
	return isDockerUnavailableError(err)
}

// containerMetadataCacheKey normalizes a Docker/container ID into the key used in
// the metadata cache, preferring the full 64-character ID when present.
func containerMetadataCacheKey(dockerID string) string {
	if full := idReg.FindString(strings.ToLower(dockerID)); full != "" {
		return full
	}
	return strings.ToLower(dockerID)
}

// mergeIntoMetadataCache writes ECS task /task container entries into dst.
// We use this instead of GET-per-container URLs: on some platforms (e.g. Windows
// ECS) those URLs return a body that does not decode as containerMetadata (e.g. a JSON
// string), while the task response Containers[] matches our struct.
func mergeIntoMetadataCache(dst map[string]containerMetadata, preload []containerMetadata) {
	for i := range preload {
		did := strings.TrimSpace(preload[i].DockerID)
		if did == "" {
			continue
		}
		key := containerMetadataCacheKey(did)
		dst[key] = preload[i]
	}
}

// parseMetadataEndpoints extracts ECS metadata endpoint URLs from a container's
// environment variable list.
func parseMetadataEndpoints(env []string) []string {
	var endpoints []string
	for _, e := range env {
		if !ecsMetadataReg.MatchString(e) {
			continue
		}
		matches := ecsMetadataReg.FindStringSubmatch(e)
		if len(matches) < 2 {
			continue
		}
		endpoints = append(endpoints, matches[1])
	}
	return endpoints
}

// ecsMetadataEndpointsFromTask queries the ECS task metadata endpoint and returns
// the discovered containers. The returned endpoint lists are empty because the
// metadata is supplied via the preload slice (the task response), avoiding
// per-container GETs that are not reliably containerMetadata documents on all platforms.
func ecsMetadataEndpointsFromTask(ctx context.Context, logger *zap.Logger) (map[string][]string, []containerMetadata, error) {
	base := strings.TrimSpace(os.Getenv(ecsContainerMetadataURIv4))
	if base == "" {
		return nil, nil, fmt.Errorf("%s is not set", ecsContainerMetadataURIv4)
	}

	taskURL := strings.TrimSuffix(base, "/") + "/task"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, taskURL, http.NoBody)
	if err != nil {
		return nil, nil, fmt.Errorf("ecs task metadata request: %w", err)
	}

	hc := &http.Client{Timeout: 15 * time.Second}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("ecs task metadata GET: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, nil, fmt.Errorf("ecs task metadata: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var task ecsTaskMetadataV4Response
	if err := json.NewDecoder(resp.Body).Decode(&task); err != nil {
		return nil, nil, fmt.Errorf("ecs task metadata decode: %w", err)
	}

	out := make(map[string][]string)
	preload := make([]containerMetadata, len(task.Containers))
	copy(preload, task.Containers)

	for i := range task.Containers {
		did := strings.TrimSpace(task.Containers[i].DockerID)
		if did == "" {
			continue
		}
		// Empty URL list: metadata comes from preload (task Containers); avoid the
		// per-container GET which is not reliably a containerMetadata JSON document.
		out[containerMetadataCacheKey(did)] = []string{}
	}

	if len(out) == 0 {
		return nil, nil, errors.New("ecs task metadata: no containers with DockerId")
	}

	logECSTaskMetadataModeOnce.Do(func() {
		logger.Info("using ECS task metadata v4 (Docker unavailable or skipped); container list refreshed periodically",
			zap.Int("container_count", len(out)))
	})
	return out, preload, nil
}

// ecsMetadataEndpointsFromTaskOrEmpty wraps ecsMetadataEndpointsFromTask, marking
// any failure as errECSTaskMetadataUnavailable so callers can preserve the cache.
func ecsMetadataEndpointsFromTaskOrEmpty(ctx context.Context, logger *zap.Logger) (map[string][]string, []containerMetadata, error) {
	m, preload, err := ecsMetadataEndpointsFromTask(ctx, logger)
	if err != nil {
		logECSTaskMetadataFallbackWarnOnce.Do(func() {
			logger.Warn("ECS task metadata unavailable; metadata cache may stay empty until ECS_CONTAINER_METADATA_URI_V4 is reachable",
				zap.Error(err))
		})
		return nil, nil, fmt.Errorf("%w: %w", errECSTaskMetadataUnavailable, err)
	}
	return m, preload, nil
}

// getEndpoints discovers per-container ECS metadata endpoints using the Docker API,
// falling back to the ECS task metadata endpoint when Docker is unavailable.
func getEndpoints(ctx context.Context, logger *zap.Logger) (map[string][]string, []containerMetadata, error) {
	m := make(map[string][]string)

	cli, err := client.New(client.WithHostFromEnv())
	if err != nil {
		if isDockerClientCreationRecoverable(err) {
			logger.Debug("failed to create Docker client; falling back to ECS task metadata", zap.Error(err))
			return ecsMetadataEndpointsFromTaskOrEmpty(ctx, logger)
		}
		return nil, nil, fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer cli.Close() // closing the client avoids leaking connections

	containers, err := cli.ContainerList(ctx, client.ContainerListOptions{})
	if err != nil {
		if isDockerUnavailableError(err) {
			logger.Debug("failed to fetch Docker containers; falling back to ECS task metadata", zap.Error(err))
			return ecsMetadataEndpointsFromTaskOrEmpty(ctx, logger)
		}
		return m, nil, fmt.Errorf("failed to fetch Docker containers: %w", err)
	}

	for i := range containers.Items {
		id := containers.Items[i].ID
		info, err := cli.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
		if err != nil {
			// A container can exit between ContainerList and ContainerInspect; skip it
			// and continue discovering the remaining containers rather than failing.
			logger.Debug("failed to inspect Docker container; skipping", zap.String("container_id", id), zap.Error(err))
			continue
		}

		if endpoints := parseMetadataEndpoints(info.Container.Config.Env); len(endpoints) > 0 {
			m[containerMetadataCacheKey(id)] = endpoints
		}
	}

	// Docker succeeded but found no task endpoints (e.g. host mismatch); try the ECS
	// task metadata endpoint when it has been injected by the agent.
	if len(m) == 0 && strings.TrimSpace(os.Getenv(ecsContainerMetadataURIv4)) != "" {
		em, preload, eerr := ecsMetadataEndpointsFromTaskOrEmpty(ctx, logger)
		if eerr != nil {
			return nil, nil, eerr
		}
		if len(em) > 0 {
			return em, preload, nil
		}
	}

	return m, nil, nil
}

// startDockerMetadataSync runs a periodic metadata sync and, when cli is non-nil,
// also re-syncs whenever Docker reports a new container. cli must be non-nil only
// when the daemon answered Ping; otherwise pass nil to avoid event-stream errors.
func startDockerMetadataSync(logger *zap.Logger, cli *client.Client, stop <-chan struct{}, syncFn func() error) {
	go func() {
		ticker := time.NewTicker(syncInterval)
		defer ticker.Stop()

		var evCh <-chan events.Message
		var errCh <-chan error
		if cli != nil {
			evCtx, cancel := context.WithCancel(context.Background())
			defer cancel() // stop the events stream when the goroutine returns
			res := cli.Events(evCtx, client.EventsListOptions{})
			evCh = res.Messages
			errCh = res.Err
		}

		for {
			select {
			case <-ticker.C:
				if err := syncFn(); err != nil {
					logger.Error("failed to sync metadata", zap.Error(err))
				}

			case event, ok := <-evCh:
				if !ok {
					evCh = nil
					continue
				}
				if event.Type != events.ContainerEventType || event.Action != "create" {
					continue
				}
				logger.Debug("new container id detected, re-syncing metadata", zap.String("id", event.Actor.ID))
				if err := syncFn(); err != nil {
					logger.Error("failed to sync metadata", zap.Error(err))
				}

			case err, ok := <-errCh:
				if !ok {
					errCh = nil
					continue
				}
				logger.Debug("docker container events stream error", zap.Error(err))

			case <-stop:
				logger.Debug("stopping metadata sync")
				return
			}
		}
	}()
}
