// Copyright  OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dockerobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/dockerobserver"

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	dtypes "github.com/docker/docker/api/types"
	dfilters "github.com/docker/docker/api/types/filters"
	"github.com/docker/go-connections/nat"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
	dcommon "github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/docker"
	docker "github.com/open-telemetry/opentelemetry-collector-contrib/internal/docker"
)

var _ (component.Extension) = (*dockerObserver)(nil)
var _ observer.Observable = (*dockerObserver)(nil)

const (
	defaultDockerAPIVersion         = 1.22
	minimalRequiredDockerAPIVersion = 1.22
	clientReconnectTimeout          = 3 * time.Second
)

type dockerObserver struct {
	logger            *zap.Logger
	config            *Config
	cancel            func()
	existingEndpoints map[string][]observer.Endpoint
	ctx               context.Context
	dClient           *docker.Client
}

// Start will instantiate required components needed by the Docker observer
func (d *dockerObserver) Start(ctx context.Context, host component.Host) error {
	dCtx, cancel := context.WithCancel(context.Background())
	d.cancel = cancel
	d.ctx = dCtx
	var err error

	// Create new Docker client
	dConfig, err := docker.NewConfig(d.config.Endpoint, d.config.Timeout, d.config.ExcludedImages, d.config.DockerAPIVersion)
	if err != nil {
		return err
	}

	d.dClient, err = docker.NewDockerClient(dConfig, d.logger)
	if err != nil {
		return fmt.Errorf("could not create docker client: %w", err)
	}

	// Load initial set of containers
	err = d.dClient.LoadContainerList(d.ctx)
	if err != nil {
		return fmt.Errorf("could not load initial list of containers: %w", err)
	}

	d.existingEndpoints = make(map[string][]observer.Endpoint)

	return nil
}

func (d *dockerObserver) Shutdown(ctx context.Context) error {
	d.cancel()
	return nil
}

// ListAndWatch provides initial state sync as well as change notification.
// Emits initial list of endpoints loaded upon extension Start. It then goes into
// a loop to sync the container cache periodically, watch for docker events,
// and notify the observer listener of endpoint changes.
func (d *dockerObserver) ListAndWatch(listener observer.Notify) {
	d.emitContainerEndpoints(listener)
	go func() {
		cacheRefreshTicker := time.NewTicker(d.config.CacheSyncInterval)
		defer cacheRefreshTicker.Stop()
		// timestamp to begin looking back for events
		lastTime := time.Now()
		var eventErr error

		// loop for re-connecting to events channel if a client error occurs
		for {
			// watchContainerEvents will only return when an error occurs, or the context is cancelled
			lastTime, eventErr = d.watchContainerEvents(listener, cacheRefreshTicker, lastTime)

			if d.ctx.Err() != nil {
				return
			}
			// Either decoding or connection error has occurred, so we should resume the event loop after
			// waiting a moment.  In cases of extended daemon unavailability this will retry until
			// collector teardown or background context is closed.
			d.logger.Error("Error watching docker container events, attempting to retry", zap.Error(eventErr))
			time.Sleep(clientReconnectTimeout)
		}
	}()
}

func (d *dockerObserver) watchContainerEvents(listener observer.Notify, cacheRefreshTicker *time.Ticker, lastEventTime time.Time) (time.Time, error) {
	// build filter list for container events
	filters := dfilters.NewArgs([]dfilters.KeyValuePair{
		{Key: "type", Value: "container"},
		{Key: "event", Value: "destroy"},
		{Key: "event", Value: "die"},
		{Key: "event", Value: "pause"},
		{Key: "event", Value: "rename"},
		{Key: "event", Value: "stop"},
		{Key: "event", Value: "start"},
		{Key: "event", Value: "unpause"},
		{Key: "event", Value: "update"},
	}...)

	opts := dtypes.EventsOptions{
		Filters: filters,
		Since:   lastEventTime.Format(time.RFC3339Nano),
	}

	eventsCh, errCh := d.dClient.Events(d.ctx, opts)

	for {
		select {
		case <-d.ctx.Done():
			return lastEventTime, nil
		case <-cacheRefreshTicker.C:
			err := d.syncContainerList(listener)
			if err != nil {
				d.logger.Error("Could not sync container cache", zap.Error(err))
			}
		case event := <-eventsCh:
			switch event.Action {
			case "destroy":
				d.updateEndpointsByContainerID(listener, event.ID, nil)
				d.dClient.RemoveContainer(event.ID)
			default:
				if c, ok := d.dClient.InspectAndPersistContainer(d.ctx, event.ID); ok {
					endpointsMap := d.endpointsForContainer(c)
					d.updateEndpointsByContainerID(listener, c.ID, endpointsMap)
				}
			}
			if event.TimeNano > lastEventTime.UnixNano() {
				lastEventTime = time.Unix(0, event.TimeNano)
			}

		case err := <-errCh:
			return lastEventTime, err
		}
	}
}

// emitContainerEndpoints notifies the listener of all changes
// by loading all current containers the client has cached and
// creating endpoints for each.
func (d *dockerObserver) emitContainerEndpoints(listener observer.Notify) {
	for _, c := range d.dClient.Containers() {
		endpointsMap := d.endpointsForContainer(c.ContainerJSON)
		d.updateEndpointsByContainerID(listener, c.ContainerJSON.ID, endpointsMap)
	}
}

// syncContainerList refreshes the client's container cache and
// uses the listener to notify endpoint changes.
func (d *dockerObserver) syncContainerList(listener observer.Notify) error {
	err := d.dClient.LoadContainerList(d.ctx)
	if err != nil {
		return err
	}
	d.emitContainerEndpoints(listener)
	return nil
}

// updateEndpointsByID uses the listener to add / remove / update endpoints by container ID.
// latestEndpointsMap is a map of latest endpoints for the given container ID.
// If an endpoint is in the cache but NOT in latestEndpoints, the endpoint will be removed
func (d *dockerObserver) updateEndpointsByContainerID(listener observer.Notify, cid string, latestEndpointsMap map[observer.EndpointID]observer.Endpoint) {
	var removedEndpoints, addedEndpoints, updatedEndpoints []observer.Endpoint

	if latestEndpointsMap != nil || len(latestEndpointsMap) != 0 {
		// map of EndpointID to endpoint to help with lookups
		existingEndpointsMap := make(map[observer.EndpointID]observer.Endpoint)
		if endpoints, ok := d.existingEndpoints[cid]; ok {
			for _, e := range endpoints {
				existingEndpointsMap[e.ID] = e
			}
		}

		// If the endpoint is present in existingEndpoints but is not
		// present in latestEndpointsMap, then it needs to be removed.
		for id, e := range existingEndpointsMap {
			if _, ok := latestEndpointsMap[id]; !ok {
				removedEndpoints = append(removedEndpoints, e)
			}
		}

		// if the endpoint is present in latestEndpointsMap, check if it exists
		// already in existingEndpoints.
		for _, e := range latestEndpointsMap {
			// If it does not exist already, it is a new endpoint. Add it.
			if existingEndpoint, ok := existingEndpointsMap[e.ID]; !ok {
				addedEndpoints = append(addedEndpoints, e)
			} else {
				// if it already exists, see if it's equal.
				// if it's not equal, update it
				// if its equal, no-op.
				if !reflect.DeepEqual(existingEndpoint, e) {
					updatedEndpoints = append(updatedEndpoints, e)
				}
			}
		}

		// reset endpoints for this container
		d.existingEndpoints[cid] = nil
		// set the current known endpoints to the latest endpoints
		for _, e := range latestEndpointsMap {
			d.existingEndpoints[cid] = append(d.existingEndpoints[cid], e)
		}
	} else {
		// if latestEndpointsMap is nil, we are removing all endpoints for the container
		removedEndpoints = append(removedEndpoints, d.existingEndpoints[cid]...)
		delete(d.existingEndpoints, cid)
	}

	if len(removedEndpoints) > 0 {
		listener.OnRemove(removedEndpoints)
	}

	if len(addedEndpoints) > 0 {
		listener.OnAdd(addedEndpoints)
	}

	if len(updatedEndpoints) > 0 {
		listener.OnChange(updatedEndpoints)
	}
}

// endpointsForContainer generates a list of observer.Endpoint given a Docker ContainerJSON.
// This function will only generate endpoints if a container is in the Running state and not Paused.
func (d *dockerObserver) endpointsForContainer(c *dtypes.ContainerJSON) map[observer.EndpointID]observer.Endpoint {
	endpointsMap := make(map[observer.EndpointID]observer.Endpoint)

	if !c.State.Running || c.State.Running && c.State.Paused {
		return endpointsMap
	}

	knownPorts := map[nat.Port]bool{}
	for k := range c.Config.ExposedPorts {
		knownPorts[k] = true
	}

	// iterate over exposed ports and try to create endpoints
	for portObj := range knownPorts {
		endpoint := d.endpointForPort(portObj, c)
		// the endpoint was not set, so we'll drop it
		if endpoint == nil {
			continue
		}
		endpointsMap[endpoint.ID] = *endpoint
	}

	if len(endpointsMap) == 0 {
		return nil
	}

	for _, e := range endpointsMap {
		s, _ := json.MarshalIndent(e, "", "\t")
		d.logger.Debug("Discovered Docker container endpoint", zap.Any("endpoint", s))
	}

	return endpointsMap
}

// endpointForPort creates an observer.Endpoint for a given port that is exposed in a Docker container.
// Each endpoint has a unique ID generated by the combination of the container.ID, container.Name,
// underlying host name, and the port number.
// Uses the user provided config settings to override certain fields.
func (d *dockerObserver) endpointForPort(portObj nat.Port, c *dtypes.ContainerJSON) *observer.Endpoint {
	endpoint := observer.Endpoint{}
	port := uint16(portObj.Int())
	proto := portObj.Proto()

	mappedPort, mappedIP := findHostMappedPort(c, portObj)
	if d.config.IgnoreNonHostBindings && mappedPort == 0 && mappedIP == "" {
		return nil
	}

	// unique ID per containerID:port
	var id observer.EndpointID
	if mappedPort != 0 {
		id = observer.EndpointID(fmt.Sprintf("%s:%d", c.ID, mappedPort))
	} else {
		id = observer.EndpointID(fmt.Sprintf("%s:%d", c.ID, port))
	}

	imageRef, err := dcommon.ParseImageName(c.Config.Image)
	if err != nil {
		d.logger.Error("could not parse container image name", zap.Error(err))
	}

	details := &observer.Container{
		Name:        strings.TrimPrefix(c.Name, "/"),
		Image:       imageRef.Repository,
		Tag:         imageRef.Tag,
		Command:     strings.Join(c.Config.Cmd, " "),
		ContainerID: c.ID,
		Transport:   portProtoToTransport(proto),
		Labels:      c.Config.Labels,
	}

	// Set our hostname based on config settings
	if d.config.UseHostnameIfPresent && c.Config.Hostname != "" {
		details.Host = c.Config.Hostname
	} else {
		// Use the IP Address of the first network we iterate over.
		// This can be made configurable if so desired.
		for _, n := range c.NetworkSettings.Networks {
			details.Host = n.IPAddress
			break
		}

		// If we still haven't gotten a host at this point and we are using
		// host bindings, just make it localhost.
		if details.Host == "" && d.config.UseHostBindings {
			details.Host = "127.0.0.1"
		}
	}

	// If we are using HostBindings & port and IP are set, use those
	if d.config.UseHostBindings && mappedPort != 0 && mappedIP != "" {
		details.Host = mappedIP
		details.Port = mappedPort
		details.AlternatePort = port
		if details.Host == "0.0.0.0" {
			details.Host = "127.0.0.1"
		}
	} else {
		details.Port = port
		details.AlternatePort = mappedPort
	}

	endpoint = observer.Endpoint{
		ID:      id,
		Target:  fmt.Sprintf("%s:%d", details.Host, details.Port),
		Details: details,
	}

	return &endpoint
}

// FindHostMappedPort returns the port number of the docker port binding to the
// underlying host, or 0 if none exists.  It also returns the mapped ip that the
// port is bound to on the underlying host, or "" if none exists.
func findHostMappedPort(c *dtypes.ContainerJSON, exposedPort nat.Port) (uint16, string) {
	bindings := c.NetworkSettings.Ports[exposedPort]

	for _, binding := range bindings {
		if port, err := nat.ParsePort(binding.HostPort); err == nil {
			return uint16(port), binding.HostIP
		}
	}
	return 0, ""
}

// Valid proto for docker containers should be tcp, udp, sctp
// https://github.com/docker/go-connections/blob/v0.4.0/nat/nat.go#L116
func portProtoToTransport(proto string) observer.Transport {
	switch proto {
	case "tcp":
		return observer.ProtocolTCP
	case "udp":
		return observer.ProtocolUDP
	}
	return observer.ProtocolUnknown
}

// newObserver creates a new docker observer extension.
func newObserver(logger *zap.Logger, config *Config) (component.Extension, error) {
	return &dockerObserver{logger: logger, config: config}, nil
}
