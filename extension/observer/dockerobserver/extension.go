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

package dockerobserver

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	dtypes "github.com/docker/docker/api/types"
	"github.com/docker/go-connections/nat"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
	docker "github.com/open-telemetry/opentelemetry-collector-contrib/internal/docker"
)

var _ (component.Extension) = (*dockerObserver)(nil)
var _ observer.Observable = (*dockerObserver)(nil)

const (
	defaultDockerAPIVersion         = 1.22
	minimalRequiredDockerAPIVersion = 1.22
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
		d.logger.Error("Could not create docker client", zap.Error(err))
		return err
	}

	// Load initial set of containers
	err = d.dClient.LoadContainerList(d.ctx)
	if err != nil {
		d.logger.Error("Could not load initial list of containers", zap.Error(err))
		return err
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
// a loop to sync the container cache periodically and change endpoints.
// TODO: Watch docker events to notify listener of changed endpoints as
// events stream
func (d *dockerObserver) ListAndWatch(listener observer.Notify) {
	d.emitContainerEndpoints(listener)

	go func() {
		ticker := time.NewTicker(d.config.CacheSyncInterval)
		defer ticker.Stop()
		for {
			select {
			case <-d.ctx.Done():
				return
			case <-ticker.C:
				err := d.syncContainerList(listener)
				if err != nil {
					d.logger.Error("Could not sync container cache", zap.Error(err))
				}
			}
		}
		// TODO: Implement event loop to watch container events to add/remove/update
		//       endpoints as they occur.
	}()
}

// emitContainerEndpoints notifies the listener of all changes
// by loading all current containers the client has cached and
// creating endpoints for each.
func (d *dockerObserver) emitContainerEndpoints(listener observer.Notify) {
	for _, c := range d.dClient.Containers() {
		cEndpoints := d.endpointsForContainer(c.ContainerJSON)
		d.updateEndpointsByContainerID(listener, c.ContainerJSON.ID, cEndpoints)
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
// latestEndpoints is the list of latest endpoints for the given container ID.
// If an endpoint is in the cache but NOT in latestEndpoints, the endpoint will be removed
func (d *dockerObserver) updateEndpointsByContainerID(listener observer.Notify, cid string, latestEndpoints []observer.Endpoint) {
	var removedEndpoints, addedEndpoints, updatedEndpoints []observer.Endpoint

	if len(latestEndpoints) != 0 {
		// Create map from ID to endpoint for lookup.
		latestEndpointsMap := make(map[observer.EndpointID]bool, len(latestEndpoints))
		for _, e := range latestEndpoints {
			latestEndpointsMap[e.ID] = true
		}
		// map of EndpointID to endpoint to help with lookups
		existingEndpointsMap := make(map[observer.EndpointID]observer.Endpoint)
		if endpoints, ok := d.existingEndpoints[cid]; ok {
			for _, e := range endpoints {
				existingEndpointsMap[e.ID] = e
			}
		}

		// If the endpoint is present in existingEndpoints but is not
		// present in latestEndpoints, then it needs to be removed.
		for id, e := range existingEndpointsMap {
			if !latestEndpointsMap[id] {
				removedEndpoints = append(removedEndpoints, e)
			}
		}

		// if the endpoint s present in latestEndpoints, check if it exists
		// already in existingEndpoints.
		for _, e := range latestEndpoints {
			// If it does not exist alreaedy, it is a new endpoint. Add it.
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

		// set the current known endpoints to the latest endpoints
		d.existingEndpoints[cid] = latestEndpoints
	} else {
		// if latestEndpoints is nil, we are removing all endpoints for the container
		removedEndpoints = append(removedEndpoints, d.existingEndpoints[cid]...)
		delete(d.existingEndpoints, cid)
	}

	if len(removedEndpoints) > 0 {
		d.logger.Info("removing endpoints")
		listener.OnRemove(removedEndpoints)
	}

	if len(addedEndpoints) > 0 {
		d.logger.Info("adding endpoints")
		listener.OnAdd(addedEndpoints)
	}

	if len(updatedEndpoints) > 0 {
		d.logger.Info("updating endpoints")
		listener.OnChange(updatedEndpoints)
	}
}

// endpointsForContainer generates a list of observer.Endpoint given a Docker ContainerJSON.
// This function will only generate endpoints if a container is in the Running state and not Paused.
func (d *dockerObserver) endpointsForContainer(c *dtypes.ContainerJSON) []observer.Endpoint {
	cEndpoints := make([]observer.Endpoint, 0)

	if !c.State.Running || c.State.Running && c.State.Paused {
		return cEndpoints
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
		cEndpoints = append(cEndpoints, *endpoint)
	}

	for _, e := range cEndpoints {
		s, _ := json.MarshalIndent(e, "", "\t")
		d.logger.Debug("Discovered Docker container endpoint", zap.Any("endpoint", s))
	}

	return cEndpoints
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

	details := &observer.Container{
		Name:        c.Name,
		Image:       c.Config.Image,
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
