// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cfgardenobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/cfgardenobserver"

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"code.cloudfoundry.org/garden"
	gardenClient "code.cloudfoundry.org/garden/client"
	gardenConnection "code.cloudfoundry.org/garden/client/connection"
	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/config"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/endpointswatcher"
)

const (
	propertiesAppIDKey     = "network.app_id"
	propertiesPortsKey     = "network.ports"
	propertiesLogConfigKey = "log_config"
	logConfigTagsKey       = "tags"
	containerStateActive   = "active"
)

type cfGardenObserver struct {
	*endpointswatcher.EndpointsWatcher
	config   *Config
	doneChan chan struct{}
	logger   *zap.Logger
	once     *sync.Once

	garden garden.Client
	cf     *client.Client

	containerMu sync.RWMutex
	containers  map[string]garden.ContainerInfo

	appMu sync.RWMutex
	apps  map[string]*resource.App
}

var _ extension.Extension = (*cfGardenObserver)(nil)

func newObserver(config *Config, logger *zap.Logger) (extension.Extension, error) {
	g := &cfGardenObserver{
		config:     config,
		logger:     logger,
		once:       &sync.Once{},
		containers: make(map[string]garden.ContainerInfo),
		apps:       make(map[string]*resource.App),
		doneChan:   make(chan struct{}),
	}
	g.EndpointsWatcher = endpointswatcher.New(g, config.RefreshInterval, logger)
	return g, nil
}

func (g *cfGardenObserver) SyncApps() error {
	g.containerMu.RLock()
	containers := g.containers
	g.containerMu.RUnlock()

	g.appMu.Lock()
	defer g.appMu.Unlock()
	g.apps = make(map[string]*resource.App)
	for _, info := range containers {
		appID, ok := info.Properties[propertiesAppIDKey]
		if !ok {
			return fmt.Errorf("container properties do not have a `%s` field, required to fetch application labels", propertiesAppIDKey)
		}

		if _, ok := g.apps[appID]; ok {
			continue
		}

		app, err := g.cf.Applications.Get(context.Background(), appID)
		if err != nil {
			return fmt.Errorf("error fetching application: %w", err)
		}
		g.apps[appID] = app
	}

	return nil
}

func (g *cfGardenObserver) App(info garden.ContainerInfo) (*resource.App, error) {
	appID, ok := info.Properties[propertiesAppIDKey]
	if !ok {
		return nil, fmt.Errorf("container properties do not have a `%s` field, required to fetch application labels", propertiesAppIDKey)
	}

	g.appMu.Lock()
	defer g.appMu.Unlock()
	app, ok := g.apps[appID]
	if ok {
		return app, nil
	}

	app, err := g.cf.Applications.Get(context.Background(), appID)
	if err != nil {
		return nil, err
	}
	g.apps[appID] = app

	return app, nil
}

func (g *cfGardenObserver) Start(_ context.Context, _ component.Host) error {
	g.garden = gardenClient.New(gardenConnection.New("unix", g.config.Garden.Endpoint))

	var err error
	g.cf, err = newCfClient(g.config.CloudFoundry)
	if err != nil {
		return err
	}

	if g.config.IncludeAppLabels {
		g.once.Do(
			func() {
				go func() {
					cacheRefreshTicker := time.NewTicker(g.config.CacheSyncInterval)
					defer cacheRefreshTicker.Stop()

					for {
						select {
						case <-g.doneChan:
							return
						case <-cacheRefreshTicker.C:
							err = g.SyncApps()
							if err != nil {
								g.logger.Error("could not sync app cache", zap.Error(err))
							}
						}
					}
				}()
			},
		)
	}

	return nil
}

func (g *cfGardenObserver) Shutdown(_ context.Context) error {
	close(g.doneChan)
	return nil
}

func (g *cfGardenObserver) ListEndpoints() []observer.Endpoint {
	var endpoints []observer.Endpoint

	containers, err := g.garden.Containers(garden.Properties{})
	if err != nil {
		g.logger.Error("could not list containers", zap.Error(err))
		return endpoints
	}

	infos := make(map[string]garden.ContainerInfo)
	for _, c := range containers {
		info, err := c.Info()
		if err != nil {
			g.logger.Error("error getting container info", zap.String("handle", c.Handle()), zap.Error(err))
			continue
		}

		if info.State != containerStateActive {
			continue
		}

		endpoints = append(endpoints, g.containerEndpoints(c.Handle(), info)...)
		infos[c.Handle()] = info
	}

	go g.updateContainerCache(infos)
	return endpoints
}

// containerEndpoints generates a list of observer.Endpoint for a container,
// this is because a container might have more than one exposed ports
func (g *cfGardenObserver) containerEndpoints(handle string, info garden.ContainerInfo) []observer.Endpoint {
	portsProp, ok := info.Properties[propertiesPortsKey]
	if !ok {
		g.logger.Error("could not discover container ports")
		return nil
	}
	ports := strings.Split(portsProp, ",")

	var app *resource.App
	var err error
	if g.config.IncludeAppLabels {
		app, err = g.App(info)
		if err != nil {
			g.logger.Error("error fetching application", zap.Error(err))
			return nil
		}
	}

	endpoints := []observer.Endpoint{}
	for _, portString := range ports {
		var port uint64
		port, err = strconv.ParseUint(portString, 10, 16)
		if err != nil {
			g.logger.Error("container port is not valid", zap.Error(err))
			continue
		}

		details := &observer.Container{
			Name:        handle,
			ContainerID: handle,
			Host:        info.ContainerIP,
			Port:        uint16(port),
			Transport:   observer.ProtocolTCP,
			Labels:      g.containerLabels(info, app),
		}

		endpoint := observer.Endpoint{
			ID:      observer.EndpointID(fmt.Sprintf("%s:%d", details.ContainerID, details.Port)),
			Target:  fmt.Sprintf("%s:%d", details.Host, details.Port),
			Details: details,
		}
		endpoints = append(endpoints, endpoint)
	}
	return endpoints
}

func (g *cfGardenObserver) containerLabels(info garden.ContainerInfo, app *resource.App) map[string]string {
	labels := make(map[string]string)
	tags, err := parseTags(info)
	if err != nil {
		g.logger.Warn("not able to parse container tags into labels", zap.Error(err))
		return nil
	}
	for k, v := range tags {
		labels[k] = v
	}

	if app != nil {
		for k, v := range app.Metadata.Labels {
			labels[k] = *v
		}
	}

	return labels
}

// The info.Properties contains a key called "log_config", which
// has contents that look like the following JSON encoded string:
//
//	{
//	  "guid": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
//	  "index": 0,
//	  "source_name": "CELL",
//	  "tags": {
//	    "app_id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
//	    "app_name": "example-app",
//	    "instance_id": "0",
//	    "organization_id": "11111111-2222-3333-4444-555555555555",
//	    "organization_name": "example-org",
//	    "process_id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
//	    "process_instance_id": "abcdef12-3456-7890-abcd-ef1234567890",
//	    "process_type": "web",
//	    "source_id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
//	    "space_id": "99999999-8888-7777-6666-555555555555",
//	    "space_name": "example-space"
//	  }
//	}
//
// We parse only the tags into a map, to be used as labels
func parseTags(info garden.ContainerInfo) (map[string]string, error) {
	logConfig, ok := info.Properties[propertiesLogConfigKey]
	if !ok {
		return nil, fmt.Errorf("container properties do not have a `%s` field", propertiesLogConfigKey)
	}

	var data map[string]any
	err := json.Unmarshal([]byte(logConfig), &data)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling logConfig: %w", err)
	}

	tags, ok := data[logConfigTagsKey].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected tags field to be a map. got=%T", data[logConfigTagsKey])
	}

	result := make(map[string]string)
	for key, value := range tags {
		if strValue, ok := value.(string); ok {
			result[key] = strValue
		}
	}

	return result, nil
}

func newCfClient(cfConfig CfConfig) (*client.Client, error) {
	var cfg *config.Config
	var err error

	switch cfConfig.Auth.Type {
	case authTypeUserPass:
		cfg, err = config.New(cfConfig.Endpoint, config.UserPassword(cfConfig.Auth.Username, cfConfig.Auth.Password))
	case authTypeClientCredentials:
		cfg, err = config.New(cfConfig.Endpoint, config.ClientCredentials(cfConfig.Auth.ClientID, cfConfig.Auth.ClientSecret))
	case authTypeToken:
		cfg, err = config.New(cfConfig.Endpoint, config.Token(cfConfig.Auth.AccessToken, cfConfig.Auth.RefreshToken))
	}

	if err != nil {
		return nil, fmt.Errorf("error creating connection to CloudFoundry API: %w", err)
	}

	c, err := client.New(cfg)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (g *cfGardenObserver) updateContainerCache(infos map[string]garden.ContainerInfo) {
	g.containerMu.Lock()
	defer g.containerMu.Unlock()
	g.containers = infos
}
