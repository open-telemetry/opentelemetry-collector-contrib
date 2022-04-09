package podmanreceiver

import (
	"context"
	"encoding/json"
	"net/url"
	"sync"
	"time"

	"go.uber.org/zap"
)

type ContainerScraper struct {
	client         PodmanClient
	containers     map[string]Container
	containersLock sync.Mutex
	logger         *zap.Logger
	config         *Config
}

func NewContainerScraper(engineClient PodmanClient, logger *zap.Logger, config *Config) *ContainerScraper {
	return &ContainerScraper{
		client:     engineClient,
		containers: make(map[string]Container),
		logger:     logger,
		config:     config,
	}
}

// LoadContainerList will load the initial running container maps for
// inspection and establishing which containers warrant stat gathering calls
// by the receiver.
func (pc *ContainerScraper) LoadContainerList(ctx context.Context) error {
	params := url.Values{}
	runningFilter := map[string][]string{
		"status": []string{"running"},
	}
	jsonFilter, err := json.Marshal(runningFilter)
	if err != nil {
		return nil
	}
	params.Add("filters", string(jsonFilter[:]))

	listCtx, cancel := context.WithTimeout(ctx, pc.config.Timeout)
	defer cancel()
	containerList, err := pc.client.list(listCtx, params)
	if err != nil {
		return err
	}

	for _, c := range containerList {
		pc.persistContainer(c)
	}
	return nil
}

func (pc *ContainerScraper) Events(ctx context.Context, options url.Values) (<-chan Event, <-chan error) {
	return pc.client.events(ctx, options)
}

func (pc *ContainerScraper) ContainerEventLoop(ctx context.Context) {
	filters := url.Values{}
	cidFilter := map[string][]string{
		"status": []string{"died", "start"},
		"type":   []string{"container"},
	}
	jsonFilter, err := json.Marshal(cidFilter)
	if err != nil {
		return
	}
	filters.Add("filters", string(jsonFilter[:]))
EVENT_LOOP:
	for {
		eventCh, errCh := pc.Events(ctx, filters)
		for {

			select {
			case <-ctx.Done():
				return
			case event := <-eventCh:
				pc.logger.Info(
					"Podman container update:",
					zap.String("id", event.ID),
					zap.String("status", event.Status),
				)
				switch event.Status {
				case "died":
					pc.logger.Debug("Podman container died:", zap.String("id", event.ID))
					pc.removeContainer(event.ID)
				case "start":
					pc.logger.Debug(
						"Podman container started:",
						zap.String("id", event.ID),
						zap.String("status", event.Status),
					)
					pc.InspectAndPersistContainer(ctx, event.ID)
				}
			case err := <-errCh:
				// We are only interested when the context hasn't been canceled since requests made
				// with a closed context are guaranteed to fail.
				if ctx.Err() == nil {
					pc.logger.Error("Error watching podman container events", zap.Error(err))
					// Either decoding or connection error has occurred, so we should resume the event loop after
					// waiting a moment.  In cases of extended daemon unavailability this will retry until
					// collector teardown or background context is closed.
					select {
					case <-time.After(3 * time.Second):
						continue EVENT_LOOP
					case <-ctx.Done():
						return
					}
				}
			}

		}
	}
}

// InspectAndPersistContainer queries inspect api and returns *containerStats and true when container should be queried for stats,
// nil and false otherwise. Persists the container in the cache if container is
// running and not excluded.
func (pc *ContainerScraper) InspectAndPersistContainer(ctx context.Context, cid string) (*Container, bool) {
	params := url.Values{}
	cidFilter := map[string][]string{
		"id": []string{cid},
	}
	jsonFilter, err := json.Marshal(cidFilter)
	if err != nil {
		return nil, false
	}
	params.Add("filters", string(jsonFilter[:]))
	listCtx, cancel := context.WithTimeout(ctx, pc.config.Timeout)
	defer cancel()
	cStats, err := pc.client.list(listCtx, params)
	if len(cStats) == 1 && err == nil {
		pc.persistContainer(cStats[0])
		return &cStats[0], true
	}
	pc.logger.Error(
		"Could not inspect updated container",
		zap.String("id", cid),
		zap.Error(err),
	)
	return nil, false
}

// FetchContainerStats will query the desired container stats
func (pc *ContainerScraper) FetchContainerStats(ctx context.Context) ([]containerStats, error) {
	params := url.Values{}
	params.Add("stream", "false")

	pc.logger.Info("Podman loaded containers", zap.Int("size", len(pc.containers)))
	for cid, _ := range pc.containers {
		pc.logger.Info("Podman fetching container", zap.String("id", cid))
		params.Add("containers", cid)
	}

	statsCtx, cancel := context.WithTimeout(ctx, pc.config.Timeout)
	defer cancel()
	return pc.client.stats(statsCtx, params)
}

func (pc *ContainerScraper) persistContainer(c Container) {
	pc.logger.Debug("Monitoring Podman container", zap.String("id", c.Id))
	pc.containersLock.Lock()
	defer pc.containersLock.Unlock()
	pc.containers[c.Id] = c
}

func (pc *ContainerScraper) removeContainer(cid string) {
	pc.containersLock.Lock()
	defer pc.containersLock.Unlock()
	delete(pc.containers, cid)
	pc.logger.Debug("Removed container from stores.", zap.String("id", cid))
}
