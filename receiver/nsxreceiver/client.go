package nsxreceiver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	nsxt "github.com/vmware/go-vmware-nsxt"
	"github.com/vmware/go-vmware-nsxt/manager"
	"go.opentelemetry.io/collector/component"

	dm "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nsxreceiver/internal/model"
)

type nsxClient struct {
	config   *Config
	driver   *nsxt.APIClient
	client   *http.Client
	endpoint *url.URL
}

var (
	errUnauthenticated = errors.New("STATUS 401, unauthenticated")
	errUnauthorized    = errors.New("STATUS 403, unauthorized")
)

const defaultMaxRetries = 3
const defaultRetryMinDelay = 5
const defaultRetryMaxDelay = 10

func newClient(c *Config, settings component.TelemetrySettings, host component.Host) (*nsxClient, error) {
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
	}, nil
}

func (c *nsxClient) LogicalSwitches(ctx context.Context) (*manager.SwitchingProfilesListResult, error) {
	body, err := c.doRequest(ctx, "/fabric/logical-switches", nil)
	if err != nil {
		return nil, err
	}
	var switches manager.SwitchingProfilesListResult
	err = json.Unmarshal(body, &switches)
	return &switches, err
}

func (c *nsxClient) Nodes(ctx context.Context) ([]dm.NodeStat, error) {
	body, err := c.doRequest(ctx, "/api/v1/transport-nodes", nil)
	if err != nil {
		return nil, err
	}
	var nodes manager.NodeListResult
	err = json.Unmarshal(body, &nodes)

	nodeIDs := []string{}
	for _, n := range nodes.Results {
		nodeIDs = append(nodeIDs, n.Id)
	}

	nodeStatsBody, err := c.doRequest(ctx, "/api/v1/fabric/nodes/status", withQuery("node_ids", strings.Join(nodeIDs, ",")))
	if err != nil {
		return nil, err
	}

	var nodeStats dm.NodeStatListResult
	err = json.Unmarshal(nodeStatsBody, &nodeStats)
	if err != nil {
		return nil, err
	}

	return nodeStats.Results, err
}

func (c *nsxClient) Interfaces(ctx context.Context, nodeID string) ([]dm.NetworkInterface, error) {
	body, err := c.doRequest(
		ctx,
		fmt.Sprintf("/api/v1/fabric/nodes/%s/network/interfaces", nodeID),
		nil,
	)
	if err != nil {
		return nil, err
	}
	var interfaces dm.NodeNetworkInterfacePropertiesListResult
	err = json.Unmarshal(body, &interfaces)

	return interfaces.Results, err
}

type requestOption func(req *http.Request)

func withQuery(key string, value string) requestOption {
	return func(req *http.Request) {
		q := req.URL.Query()
		q.Add(key, value)
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
		op(req)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return io.ReadAll(resp.Body)
	}

	_, err = io.ReadAll(resp.Body)
	switch resp.StatusCode {
	case 401:
		return nil, errUnauthenticated
	case 403:
		return nil, errUnauthorized
	default:
		return nil, fmt.Errorf("got non 200 status code %d", resp.StatusCode)
	}
}
