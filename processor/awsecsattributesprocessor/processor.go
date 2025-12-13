package awsecsattributesprocessor

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/client"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/awsecsattributesprocessor/internal/metadata"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"
)

// MetadataGetter retrieves metadata for a container ID
type MetadataGetter func(key string) (Metadata, error)

// DataProcessor processes telemetry data of type T
type DataProcessor[T any] interface {
	Process(ctx context.Context, logger *zap.Logger, cfg *Config, getMeta MetadataGetter, data T) error
	Forward(ctx context.Context, data T) error
}

type processorBase[T any, D DataProcessor[T]] struct {
	sync.RWMutex

	// context
	ctx context.Context

	// endpoints map[string][]string
	metadata map[string]Metadata

	// metadata age
	metadataAge time.Time

	// docker container labels
	// note: we record the container labels here since
	// the ECS Metadata does not capture labels defined at the
	// container level
	labels map[string]map[string]string

	// logger
	logger *zap.Logger

	// endpoints -  function used to fetch metadata endpoints
	endpoints endpointsFn

	// containerDataFn - function used to fetch container data
	containerdata containerDataFn

	// data processor
	dataProcessor D

	// config
	cfg *Config

	// docker client
	dockerClient *client.Client

	stop chan struct{}
}

func newProcessorBase[T any, D DataProcessor[T]](
	ctx context.Context,
	logger *zap.Logger,
	cfg *Config,
	dataProcessor D,
	endpoints endpointsFn,
	containerdata containerDataFn,
) *processorBase[T, D] {
	return &processorBase[T, D]{
		ctx:           ctx,
		logger:        logger,
		cfg:           cfg,
		dataProcessor: dataProcessor,
		endpoints:     endpoints,
		containerdata: containerdata,

		// initialise
		metadata:    make(map[string]Metadata),
		metadataAge: time.Now(),
		labels:      make(map[string]map[string]string),
		stop:        make(chan struct{}),
	}
}

// get - synchronises metadata for container IDs that are not found in the internal cache
func (p *processorBase[T, D]) get(key string) (Metadata, error) {
	// initial check for metadata
	p.RLock()
	val, ok := p.metadata[key]
	p.RUnlock()

	if ok {
		return val, nil
	}

	// sync metadata if not found
	if err := runSync(p); err != nil {
		return Metadata{}, fmt.Errorf("failed to sync metadata: %w", err)
	}

	// check for metadata again
	p.RLock()
	val, ok = p.metadata[key]
	p.RUnlock()

	if !ok {
		return val, fmt.Errorf("metadata not found for container id after sync: %s", key)
	}
	return val, nil
}

func (p *processorBase[T, D]) syncMetadata(_ context.Context, endpoints map[string][]string) error {
	p.Lock()
	defer p.Unlock()

	for k, v := range endpoints {
		if _, ok := p.metadata[k]; ok {
			continue // if metadata already exists, skip
		}

		var metadata Metadata
		resp, err := http.Get(v[0]) // use the 1st available endpoint
		if err != nil {
			p.logger.Error("failed while calling metadata endpoint", zap.String("container_id", k), zap.Error(err))
			continue
		}

		if err = json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
			p.logger.Error("failed to decode metadata", zap.String("container_id", k), zap.Error(err))
			continue
		}

		p.metadata[k] = metadata
	}

	// remove keys that don't exist in current endpoint view
	if time.Since(p.metadataAge) > time.Duration(p.cfg.CacheTTL)*time.Second {
		p.logger.Debug("removing stale metadata")
		for k := range p.metadata {
			if _, ok := endpoints[k]; !ok {
				delete(p.metadata, k)
			}
		}

		p.metadataAge = time.Now()
	}

	return nil
}

// interface functions

func (p *processorBase[T, D]) Start(ctx context.Context, _ component.Host) error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		p.logger.Sugar().Errorf("failed to intial docker API client: %w", err)
		return err
	}

	p.dockerClient = cli

	// initial sync
	if err := runSync(p); err != nil {
		p.logger.Sugar().Errorf("failed to sync metadata: %w", err)
		return err
	}

	go func() {
		ctx := context.Background()
		ticker := time.NewTicker(time.Second * 60)

		dockerEvents, errors := p.dockerClient.Events(ctx, events.ListOptions{})

		for {
			select {
			case <-ticker.C:
				if err := runSync(p); err != nil {
					p.logger.Sugar().Errorf("failed to sync metadata: %w", err)
				}

			case event := <-dockerEvents:
				if !(event.Type == events.ContainerEventType && event.Action == "create") {
					continue
				}

				p.logger.Debug("new container id detected, re-syncing metadata", zap.String("id", event.ID))
				if err := runSync(p); err != nil {
					p.logger.Sugar().Errorf("failed to sync metadata: %w", err)
				}

			case err := <-errors:
				if err != nil {
					p.logger.Sugar().Errorf("error received from docker container events: %w", err)
				}

			case <-p.stop:
				p.logger.Debug("stopping metadata sync")
				return
			}
		}
	}()

	return err
}

func (p *processorBase[T, D]) Shutdown(context.Context) error {
	p.stop <- struct{}{}
	close(p.stop)
	if p.dockerClient != nil {
		return p.dockerClient.Close()
	}
	return nil
}

func (*processorBase[T, D]) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func (p *processorBase[T, D]) Consume(ctx context.Context, data T) error {
	if err := p.dataProcessor.Process(ctx, p.logger, p.cfg, p.get, data); err != nil {
		return err
	}
	return p.dataProcessor.Forward(ctx, data)
}

func runSync[T any, D DataProcessor[T]](p *processorBase[T, D]) error {
	endpoints, err := p.endpoints(p.logger, p.ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch metadata endpoints: %w", err)
	}

	p.logger.Debug("updating endpoints", zap.String("processor", metadata.Type.String()))
	if err := p.syncMetadata(p.ctx, endpoints); err != nil {
		return fmt.Errorf("%s failed to update metadata endpoints: %w", metadata.Type, err)
	}

	p.logger.Debug("number of containers with detected metadata endpoints", zap.Int("count", len(endpoints)))
	return nil
}

func getContainerData(ctx context.Context) (map[string]ContainerData, error) {
	m := make(map[string]ContainerData)

	// Initialize Docker client
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return m, err
	}

	defer cli.Close()

	// Get the list of running containers
	containers, err := cli.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		return m, err
	}

	for _, container := range containers {
		// Fetch detailed container information
		containerInfo, err := cli.ContainerInspect(ctx, container.ID)
		if err != nil {
			return m, err
		}

		containerData := ContainerData{
			Container: container,
		}

		// use regex to match ECS_CONTAINER_METADATA_URI_V4 and ECS_CONTAINER_METADATA_URI
		for _, env := range containerInfo.Config.Env {
			if !ecsMetadataReg.MatchString(env) {
				continue
			}

			matches := ecsMetadataReg.FindStringSubmatch(env)
			if len(matches) < 2 {
				continue
			}

			containerData.Endpoints = append(containerData.Endpoints, matches[1])
		}

		// only record if there are endpoints
		if len(containerData.Endpoints) > 0 {
			m[container.ID] = containerData
		}
	}

	return m, nil
}
