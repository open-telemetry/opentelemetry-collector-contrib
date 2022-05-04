// Copyright  The OpenTelemetry Authors
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

package nsxreceiver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	dm "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nsxreceiver/internal/model"
)

var _ (Client) = &nsxClient{}

// Client is a way of interacting with the NSX REST API
type Client interface {
	TransportNodes(ctx context.Context) ([]dm.TransportNode, error)
	ClusterNodes(ctx context.Context) ([]dm.ClusterNode, error)
	NodeStatus(ctx context.Context, nodeID string, class nodeClass) (*dm.NodeStatus, error)
	Interfaces(ctx context.Context, nodeID string, class nodeClass) ([]dm.NetworkInterface, error)
	InterfaceStatus(ctx context.Context, nodeID, interfaceID string, class nodeClass) (*dm.NetworkInterfaceStats, error)
}

type nsxClient struct {
	config   *Config
	client   *http.Client
	endpoint *url.URL
	logger   *zap.Logger
}

var (
	errUnauthorized = errors.New("STATUS 403, unauthorized")
)

func newClient(c *Config, settings component.TelemetrySettings, host component.Host, logger *zap.Logger) (*nsxClient, error) {
	client, err := c.HTTPClientSettings.ToClient(host.GetExtensions(), settings)
	if err != nil {
		return nil, err
	}

	endpoint, err := url.Parse(c.Endpoint)
	if err != nil {
		return nil, err
	}

	return &nsxClient{
		config:   c,
		client:   client,
		endpoint: endpoint,
		logger:   logger,
	}, nil
}

func (c *nsxClient) TransportNodes(ctx context.Context) ([]dm.TransportNode, error) {
	body, err := c.doRequest(
		ctx,
		"/api/v1/transport-nodes",
		withDefaultHeaders(),
	)
	if err != nil {
		return nil, err
	}
	var nodes dm.TransportNodeList
	err = json.Unmarshal(body, &nodes)
	return nodes.Results, err
}

func (c *nsxClient) ClusterNodes(ctx context.Context) ([]dm.ClusterNode, error) {
	body, err := c.doRequest(
		ctx,
		"/api/v1/cluster/nodes",
		withDefaultHeaders(),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to get cluster nodes: %w", err)
	}
	var nodes dm.ClusterNodeList
	err = json.Unmarshal(body, &nodes)

	return nodes.Results, err
}

func (c *nsxClient) NodeStatus(ctx context.Context, nodeID string, class nodeClass) (*dm.NodeStatus, error) {
	body, err := c.doRequest(
		ctx,
		c.nodeStatusEndpoint(class, nodeID),
		withDefaultHeaders(),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to get a node's status from the REST API: %w", err)
	}

	switch class {
	case transportClass:
		var nodeStatus dm.TransportNodeStatus
		err = json.Unmarshal(body, &nodeStatus)
		return &nodeStatus.NodeStatus, err
	default:
		var nodeStatus dm.NodeStatus
		err = json.Unmarshal(body, &nodeStatus)
		return &nodeStatus, err
	}

}

func (c *nsxClient) Interfaces(
	ctx context.Context,
	nodeID string,
	class nodeClass,
) ([]dm.NetworkInterface, error) {
	body, err := c.doRequest(
		ctx,
		c.interfacesEndpoint(class, nodeID),
		withDefaultHeaders(),
	)
	if err != nil {
		return nil, err
	}
	var interfaces dm.NodeNetworkInterfacePropertiesListResult
	err = json.Unmarshal(body, &interfaces)

	return interfaces.Results, err
}

func (c *nsxClient) InterfaceStatus(
	ctx context.Context,
	nodeID, interfaceID string,
	class nodeClass,
) (*dm.NetworkInterfaceStats, error) {
	body, err := c.doRequest(
		ctx,
		c.interfaceStatusEndpoint(class, nodeID, interfaceID),
		withDefaultHeaders(),
	)

	if err != nil {
		return nil, fmt.Errorf("unable to get interface stats: %w", err)
	}
	var interfaceStats dm.NetworkInterfaceStats
	err = json.Unmarshal(body, &interfaceStats)
	return &interfaceStats, err
}

type requestOption func(req *http.Request) *http.Request

func withDefaultHeaders() requestOption {
	return func(req *http.Request) *http.Request {
		h := req.Header
		h.Add("User-Agent", "opentelemetry-collector")
		h.Add("Accept", "application/json")
		h.Add("Connection", "keep-alive")
		return req
	}
}

func (c *nsxClient) doRequest(ctx context.Context, path string, options ...requestOption) ([]byte, error) {
	endpoint, err := c.endpoint.Parse(path)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.config.Username, c.config.Password)

	for _, op := range options {
		req = op(req)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return io.ReadAll(resp.Body)
	}

	body, _ := io.ReadAll(resp.Body)
	switch resp.StatusCode {
	case 403:
		return nil, errUnauthorized
	default:
		c.logger.Info(fmt.Sprintf("%v", req))
		return nil, fmt.Errorf("got non 200 status code %d: %w, %s", resp.StatusCode, err, string(body))
	}
}

func (c *nsxClient) nodeStatusEndpoint(class nodeClass, nodeID string) string {
	switch class {
	case transportClass:
		return fmt.Sprintf("/api/v1/transport-nodes/%s/status", nodeID)
	default:
		return fmt.Sprintf("/api/v1/cluster/nodes/%s/status", nodeID)
	}
}

func (c *nsxClient) interfacesEndpoint(class nodeClass, nodeID string) string {
	switch class {
	case transportClass:
		return fmt.Sprintf("/api/v1/transport-nodes/%s/network/interfaces", nodeID)
	default:
		return fmt.Sprintf("/api/v1/cluster/nodes/%s/network/interfaces", nodeID)
	}
}

func (c *nsxClient) interfaceStatusEndpoint(class nodeClass, nodeID, interfaceID string) string {
	switch class {
	case transportClass:
		return fmt.Sprintf("/api/v1/transport-nodes/%s/network/interfaces/%s/stats", nodeID, interfaceID)
	default:
		return fmt.Sprintf("/api/v1/cluster/nodes/%s/network/interfaces/%s/stats", nodeID, interfaceID)
	}
}
