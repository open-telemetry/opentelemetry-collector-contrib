// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dockerobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/dockerobserver"

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	ctypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/endpointswatcher"
	dcommon "github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/docker"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/docker"
)

var (
	defaultDockerAPIVersion         = "1.44"
	minimumRequiredDockerAPIVersion = docker.MustNewAPIVersion(defaultDockerAPIVersion)
)

var (
	_ extension.Extension = (*dockerObserver)(nil)
	_ observer.Observable = (*dockerObserver)(nil)
)

type dockerObserver struct {
	*endpointswatcher.EndpointsWatcher
	logger  *zap.Logger
	config  *Config
	cancel  context.CancelFunc
	wg      errgroup.Group
	dClient *docker.Client
}

// newObserver creates a new docker observer extension.
func newObserver(logger *zap.Logger, config *Config) (extension.Extension, error) {
	d := &dockerObserver{
		logger: logger, config: config,
		cancel: func() {
			// Safe value provided on initialisation
		},
	}
	d.EndpointsWatcher = endpointswatcher.New(d, time.Second, logger)
	return d, nil
}

// Start will instantiate required components needed by the Docker observer
func (d *dockerObserver) Start(ctx context.Context, _ component.Host) error {
	dCtx, cancel := context.WithCancel(context.Background())
	d.cancel = cancel

	// Create new Docker client
	dConfig := docker.NewConfig(d.config.Endpoint, d.config.Timeout, d.config.ExcludedImages, d.config.DockerAPIVersion)

	var err error
	d.dClient, err = docker.NewDockerClient(dConfig, d.logger)
	if err != nil {
		return fmt.Errorf("could not create docker client: %w", err)
	}

	err = d.dClient.LoadContainerList(ctx)
	if err != nil {
		return err
	}

	d.wg.Go(func() error {
		cacheRefreshTicker := time.NewTicker(d.config.CacheSyncInterval)
		defer cacheRefreshTicker.Stop()

		for done := false; !done; {
			select {
			case <-dCtx.Done():
				done = true
			case <-cacheRefreshTicker.C:
				err = d.dClient.LoadContainerList(dCtx)
				if err != nil {
					d.logger.Error("Could not sync container cache", zap.Error(err))
				}
			}
		}
		return nil
	})

	d.wg.Go(func() error {
		d.dClient.ContainerEventLoop(dCtx)
		return nil
	})

	return nil
}

func (d *dockerObserver) Shutdown(_ context.Context) error {
	d.StopListAndWatch()
	d.cancel()
	return d.wg.Wait()
}

func (d *dockerObserver) ListEndpoints() []observer.Endpoint {
	var endpoints []observer.Endpoint
	for _, container := range d.dClient.Containers() {
		endpoints = append(endpoints, d.containerEndpoints(container.InspectResponse)...)
	}
	return endpoints
}

// containerEndpoints generates a list of observer.Endpoint given a Docker InspectResponse.
// This function will only generate endpoints if a container is in the Running state and not Paused.
func (d *dockerObserver) containerEndpoints(c *ctypes.InspectResponse) []observer.Endpoint {
	if !c.State.Running || c.State.Running && c.State.Paused {
		return nil
	}

	knownPorts := map[network.Port]bool{}
	for k := range c.Config.ExposedPorts {
		knownPorts[k] = true
	}
	endpoints := make([]observer.Endpoint, 0, len(knownPorts))
	// iterate over exposed ports and try to create endpoints
	for portObj := range knownPorts {
		endpoint := d.endpointForPort(portObj, c)
		// the endpoint was not set, so we'll drop it
		if endpoint == nil {
			continue
		}
		endpoints = append(endpoints, *endpoint)
	}

	if d.config.IncludeAllContainers {
		if endpoint := d.endpointForContainer(c); endpoint != nil {
			endpoints = append(endpoints, *endpoint)
		}
	}

	return endpoints
}

// endpointForContainer creates a port-less observer.Endpoint for the container itself.
// It is emitted when IncludeAllContainers is enabled so that receiver_creator rules
// matching `type == "container"` can attach receivers to every running container,
// including those that expose no ports.
func (d *dockerObserver) endpointForContainer(c *ctypes.InspectResponse) *observer.Endpoint {
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
		Transport:   observer.ProtocolUnknown,
		Labels:      c.Config.Labels,
		Host:        d.containerHost(c),
	}

	return &observer.Endpoint{
		ID:      observer.EndpointID(c.ID),
		Target:  details.Host,
		Details: details,
	}
}

// containerHost resolves a host string for a container without considering port bindings.
func (d *dockerObserver) containerHost(c *ctypes.InspectResponse) string {
	if d.config.UseHostnameIfPresent && c.Config.Hostname != "" {
		return c.Config.Hostname
	}
	for _, n := range c.NetworkSettings.Networks {
		if n.IPAddress.IsValid() {
			return n.IPAddress.String()
		}
		break
	}
	if d.config.UseHostBindings {
		return "127.0.0.1"
	}
	return ""
}

// endpointForPort creates an observer.Endpoint for a given port that is exposed in a Docker container.
// Each endpoint has a unique ID generated by the combination of the container.ID, container.Name,
// underlying host name, and the port number.
// Uses the user provided config settings to override certain fields.
func (d *dockerObserver) endpointForPort(portObj network.Port, c *ctypes.InspectResponse) *observer.Endpoint {
	endpoint := observer.Endpoint{}
	port := portObj.Num()
	proto := string(portObj.Proto())

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
			if n.IPAddress.IsValid() {
				details.Host = n.IPAddress.String()
			}
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
func findHostMappedPort(c *ctypes.InspectResponse, exposedPort network.Port) (uint16, string) {
	bindings := c.NetworkSettings.Ports[exposedPort]

	for _, binding := range bindings {
		if port, err := strconv.ParseUint(binding.HostPort, 10, 16); err == nil {
			hostIP := ""
			if binding.HostIP.IsValid() {
				hostIP = binding.HostIP.String()
			}
			return uint16(port), hostIP
		}
	}
	return 0, ""
}

// Valid proto for docker containers should be tcp, udp, sctp
// https://pkg.go.dev/github.com/moby/moby/api/types/network@v1.54.1#IPProtocol
func portProtoToTransport(proto string) observer.Transport {
	switch proto {
	case "tcp":
		return observer.ProtocolTCP
	case "udp":
		return observer.ProtocolUDP
	}
	return observer.ProtocolUnknown
}
