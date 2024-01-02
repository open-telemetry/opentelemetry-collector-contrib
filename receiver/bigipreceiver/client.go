// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package bigipreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/bigipreceiver"

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/bigipreceiver/internal/models"
)

const (
	// loginPath is the path to the login endpoint
	loginPath = "/mgmt/shared/authn/login"
	// virtualServersPath is the path to the virtual servers endpoint
	virtualServersPath = "/mgmt/tm/ltm/virtual"
	// virtualServersStatsPath is the path to the virtual servers statistics endpoint
	virtualServersStatsPath = "/mgmt/tm/ltm/virtual/stats"
	// poolsStatsPath is the path to the pools statistics endpoint
	poolsStatsPath = "/mgmt/tm/ltm/pool/stats"
	// nodesStatsPath is the path to the nodes statistics endpoint
	nodesStatsPath = "/mgmt/tm/ltm/node/stats"
	// poolMembersStatsPathSuffix is the suffix added onto an individual pool's statistics endpoint
	poolMembersStatsPathSuffix = "/members/stats"
)

// custom errors
var (
	errCollectedNoPoolMembers = errors.New(`all pool member requests have failed`)
)

// client is used for retrieving data about a Big-IP environment
type client interface {
	// HasToken checks if the client currently has an auth token
	HasToken() bool
	// GetNewToken must be called initially as it retrieves and sets an auth token for future calls
	GetNewToken(ctx context.Context) error
	// GetVirtualServers retrieves data for all LTM virtual servers in a Big-IP environment
	GetVirtualServers(ctx context.Context) (*models.VirtualServers, error)
	// GetPools retrieves data for all LTM pools in a Big-IP environment
	GetPools(ctx context.Context) (*models.Pools, error)
	// GetPoolMembers retrieves data for all LTM pool members in a Big-IP environment
	GetPoolMembers(ctx context.Context, pools *models.Pools) (*models.PoolMembers, error)
	// GetNodes retrieves data for all LTM nodes in a Big-IP environment
	GetNodes(ctx context.Context) (*models.Nodes, error)
}

// bigipClient implements the client interface and retrieves data through the iControl REST API
type bigipClient struct {
	client       *http.Client
	hostEndpoint string
	creds        bigipCredentials
	token        string
	logger       *zap.Logger
}

// bigipCredentials stores the username and password needed to retrieve an access token from the iControl REST API
type bigipCredentials struct {
	username string
	password string
}

// Verify bigipClient implements client interface
var _ client = (*bigipClient)(nil)

// newClient creates an initialized client (but with no token)
func newClient(cfg *Config, host component.Host, settings component.TelemetrySettings, logger *zap.Logger) (client, error) {
	httpClient, err := cfg.ToClient(host, settings)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP Client: %w", err)
	}

	return &bigipClient{
		client:       httpClient,
		hostEndpoint: cfg.Endpoint,
		creds: bigipCredentials{
			username: cfg.Username,
			password: string(cfg.Password),
		},
		logger: logger,
	}, nil
}

// HasToken checks to see if an auth token has been set for the client
func (c *bigipClient) HasToken() bool {
	return c.token != ""
}

// GetNewToken makes an appropriate call to the iControl REST login endpoint and sets the returned token on the bigipClient
func (c *bigipClient) GetNewToken(ctx context.Context) error {
	var tokenDetails *models.TokenDetails

	if err := c.post(ctx, loginPath, &tokenDetails); err != nil {
		c.logger.Debug("Failed to retrieve api token", zap.Error(err))
		return err
	}

	c.token = tokenDetails.Token.Token
	return nil
}

// GetVirtualServers makes calls to both the standard and statistics version of the virtual servers endpoint.
// It combines this info into one object and returns it.
func (c *bigipClient) GetVirtualServers(ctx context.Context) (*models.VirtualServers, error) {
	// get standard Virtual Server details
	var virtualServers *models.VirtualServers

	if err := c.get(ctx, virtualServersStatsPath, &virtualServers); err != nil {
		c.logger.Debug("Failed to retrieve virtual servers", zap.Error(err))
		return nil, err
	}

	// get statistic virtual server details and combine them
	var virtualServersDetails *models.VirtualServersDetails

	if err := c.get(ctx, virtualServersPath, &virtualServersDetails); err != nil {
		c.logger.Warn("Failed to retrieve virtual servers properties", zap.Error(err))
		return virtualServers, nil
	}

	return addVirtualServerPoolDetails(virtualServers, virtualServersDetails), nil
}

// GetPools makes a call the statistics version of the pools endpoint and returns the data.
func (c *bigipClient) GetPools(ctx context.Context) (*models.Pools, error) {
	var pools *models.Pools

	if err := c.get(ctx, poolsStatsPath, &pools); err != nil {
		c.logger.Debug("Failed to retrieve pools", zap.Error(err))
		return nil, err
	}

	return pools, nil
}

// GetPoolMembers takes in a list of all Pool data. It then iterates over this list to make a call to the statistics version
// of each pool's pool members endpoint. It accumulates all of this data into a single pool members object and returns it.
func (c *bigipClient) GetPoolMembers(ctx context.Context, pools *models.Pools) (*models.PoolMembers, error) {
	var (
		poolMembers         *models.PoolMembers
		combinedPoolMembers *models.PoolMembers
	)
	collectedPoolMembers := false

	var errors []error
	// for each pool get pool member info and aggregate it into a single spot
	for poolURL := range pools.Entries {
		poolMemberPath := strings.TrimPrefix(poolURL, "https://localhost")
		poolMemberPath = strings.TrimSuffix(poolMemberPath, "/stats") + poolMembersStatsPathSuffix

		if err := c.get(ctx, poolMemberPath, &poolMembers); err != nil {
			errors = append(errors, err)
			c.logger.Warn("Failed to retrieve all pool members", zap.Error(err))
		} else {
			combinedPoolMembers = combinePoolMembers(combinedPoolMembers, poolMembers)
			collectedPoolMembers = true
		}
	}

	combinedErr := multierr.Combine(errors...)

	if combinedErr != nil && !collectedPoolMembers {
		return nil, errCollectedNoPoolMembers
	}

	return combinedPoolMembers, combinedErr
}

// GetNodes makes a call the statistics version of the nodes endpoint and returns the data.
func (c *bigipClient) GetNodes(ctx context.Context) (nodes *models.Nodes, err error) {
	if err = c.get(ctx, nodesStatsPath, &nodes); err != nil {
		c.logger.Debug("Failed to retrieve nodes", zap.Error(err))
		return nil, err
	}

	return nodes, nil
}

// post makes a POST request for the passed in path and stores result in the respObj
func (c *bigipClient) post(ctx context.Context, path string, respObj any) error {
	// Construct endpoint and create request
	url := c.hostEndpoint + path
	postBody, _ := json.Marshal(map[string]string{
		"username":          c.creds.username,
		"password":          c.creds.password,
		"loginProviderName": "tmos",
	})
	requestBody := bytes.NewBuffer(postBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, requestBody)
	if err != nil {
		return fmt.Errorf("failed to create post request for path %s: %w", path, err)
	}

	return c.makeHTTPRequest(req, respObj)
}

// get makes a GET request (with token in header) for the passed in path and stores result in the respObj
func (c *bigipClient) get(ctx context.Context, path string, respObj any) error {
	// Construct endpoint and create request
	url := c.hostEndpoint + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	req.Header.Add("X-F5-Auth-Token", c.token)
	if err != nil {
		return fmt.Errorf("failed to create get request for path %s: %w", path, err)
	}

	return c.makeHTTPRequest(req, respObj)
}

// makeHTTPRequest makes the request and decodes the body into the respObj on a 200 Status
func (c *bigipClient) makeHTTPRequest(req *http.Request, respObj any) (err error) {
	// Make request
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make http request: %w", err)
	}

	// Defer body close
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			c.logger.Warn("failed to close response body", zap.Error(closeErr))
		}
	}()

	// Check for OK status code
	if err = c.checkHTTPStatus(resp); err != nil {
		return err
	}

	// Decode the payload into the passed in response object
	if err := json.NewDecoder(resp.Body).Decode(respObj); err != nil {
		return fmt.Errorf("failed to decode response payload: %w", err)
	}

	return nil
}

// checkHTTPStatus returns an error if the response status is != 200
func (c *bigipClient) checkHTTPStatus(resp *http.Response) (err error) {
	if resp.StatusCode != http.StatusOK {
		c.logger.Debug("Big-IP API non-200", zap.Error(err), zap.Int("status_code", resp.StatusCode))

		// Attempt to extract the error payload
		payloadData, err := io.ReadAll(resp.Body)
		if err != nil {
			c.logger.Debug("failed to read payload error message", zap.Error(err))
		} else {
			c.logger.Debug("Big-IP API Error", zap.ByteString("api_error", payloadData))
		}

		return fmt.Errorf("non 200 code returned %d", resp.StatusCode)
	}

	return nil
}

// combinePoolMembers takes two PoolMembers and returns an aggregate of them both
func combinePoolMembers(poolMembersA *models.PoolMembers, poolMembersB *models.PoolMembers) *models.PoolMembers {
	var aSize int
	if poolMembersA != nil {
		aSize = len(poolMembersA.Entries)
	}

	var bSize int
	if poolMembersB != nil {
		bSize = len(poolMembersB.Entries)
	}

	totalSize := aSize + bSize
	if totalSize == 0 {
		return &models.PoolMembers{}
	}

	combinedPoolMembers := models.PoolMembers{Entries: make(map[string]models.PoolMemberStats, totalSize)}

	if poolMembersA != nil {
		for url, data := range poolMembersA.Entries {
			combinedPoolMembers.Entries[url] = data
		}
	}
	if poolMembersB != nil {
		for url, data := range poolMembersB.Entries {
			combinedPoolMembers.Entries[url] = data
		}
	}

	return &combinedPoolMembers
}

// addVirtualServerPoolDetails takes in VirtualServers and VirtualServersDetails, matches the data, and combines them into a returned VirtualServers
func addVirtualServerPoolDetails(virtualServers *models.VirtualServers, virtualServersDetails *models.VirtualServersDetails) *models.VirtualServers {
	vSize := len(virtualServers.Entries)
	if vSize == 0 {
		return &models.VirtualServers{}
	}

	combinedVirtualServers := models.VirtualServers{Entries: make(map[string]models.VirtualServerStats, vSize)}

	for virtualServerURL, entry := range virtualServers.Entries {
		combinedVirtualServers.Entries[virtualServerURL] = entry
	}

	// for each item in VirtualServersDetails match it with the entry in VirtualServers, combine it, and add it to the combined data object
	for _, item := range virtualServersDetails.Items {
		parts := strings.Split(item.SelfLink, "?")
		entryKey := parts[0] + "/stats"
		if entryValue, ok := combinedVirtualServers.Entries[entryKey]; ok {
			entryValue.NestedStats.Entries.PoolName.Description = item.PoolName
			combinedVirtualServers.Entries[entryKey] = entryValue
		}
	}

	return &combinedVirtualServers
}
