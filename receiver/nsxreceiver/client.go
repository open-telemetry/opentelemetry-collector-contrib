package nsxreceiver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	nsxt "github.com/vmware/go-vmware-nsxt"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	dm "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nsxreceiver/internal/model"
)

type nsxClient struct {
	config   *Config
	driver   *nsxt.APIClient
	client   *http.Client
	endpoint *url.URL
	logger   *zap.Logger
}

var (
	errUnauthenticated = errors.New("STATUS 401, unauthenticated")
	errUnauthorized    = errors.New("STATUS 403, unauthorized")
)

const defaultMaxRetries = 3
const defaultRetryMinDelay = 5
const defaultRetryMaxDelay = 10

func newClient(c *Config, settings component.TelemetrySettings, host component.Host, logger *zap.Logger) (*nsxClient, error) {
	client, err := c.MetricsConfig.HTTPClientSettings.ToClient(host.GetExtensions(), settings)
	if err != nil {
		return nil, err
	}

	endpoint, err := url.Parse(c.MetricsConfig.Endpoint)
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

func (c *nsxClient) NodeStatus(ctx context.Context, nodeID string) (*dm.NodeStatus, error) {
	body, err := c.doRequest(
		ctx,
		fmt.Sprintf("/api/v1/transport-nodes/%s/status", nodeID),
		withDefaultHeaders(),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to get a node's status from the REST API")
	}

	var nodestatus dm.TransportNodeStatus
	err = json.Unmarshal(body, &nodestatus)
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal result of the node status request: %w", err)
	}

	c.logger.Info(fmt.Sprintf("here is the node status: %v", nodestatus.NodeStatus.SystemStatus))

	return &nodestatus.NodeStatus, nil

}

func (c *nsxClient) Interfaces(ctx context.Context, nodeID string) ([]dm.NetworkInterface, error) {
	body, err := c.doRequest(
		ctx,
		fmt.Sprintf("/api/v1/fabric/nodes/%s/network/interfaces", nodeID),
		withDefaultHeaders(),
	)
	if err != nil {
		return nil, err
	}
	var interfaces dm.NodeNetworkInterfacePropertiesListResult
	err = json.Unmarshal(body, &interfaces)

	return interfaces.Results, err
}

type requestOption func(req *http.Request) *http.Request

func withQuery(key string, value string) requestOption {
	return func(req *http.Request) *http.Request {
		q := req.URL.Query()
		q.Add(key, value)
		req.URL.RawQuery = q.Encode()
		return req
	}
}

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
	req.SetBasicAuth(c.config.MetricsConfig.Username, c.config.MetricsConfig.Password)

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
	case 401:
		return nil, errUnauthenticated
	case 403:
		return nil, errUnauthorized
	default:
		c.logger.Info(fmt.Sprintf("%v", req))
		return nil, fmt.Errorf("got non 200 status code %d: %w, %s", resp.StatusCode, err, string(body))
	}
}
