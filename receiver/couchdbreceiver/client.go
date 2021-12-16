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

package couchdbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/couchdbreceiver"

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
)

const nodeNamesPath = "/_membership"

// CouchDBClient defines the basic HTTP client interface.
type CouchDBClient interface {
	Get(path string) ([]byte, error)
	GetNodeNames() ([]string, error)
	GetStats(nodeName string) (map[string]interface{}, error)
}

var _ CouchDBClient = (*couchDBClient)(nil)

type couchDBClient struct {
	client *http.Client
	cfg    *Config
	logger *zap.Logger
}

// NewCouchDBClient creates a new client to make requests for the CouchDB receiver.
func NewCouchDBClient(cfg *Config, host component.Host, logger *zap.Logger) (CouchDBClient, error) {
	client, err := cfg.ToClient(host.GetExtensions())
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP Client: %w", err)
	}

	return &couchDBClient{
		client: client,
		cfg:    cfg,
		logger: logger,
	}, nil
}

// Get issues an authorized Get requests to the specified url.
func (cc *couchDBClient) Get(path string) ([]byte, error) {
	req, err := cc.buildReq(path)
	if err != nil {
		return nil, err
	}

	resp, err := cc.client.Do(req)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err = resp.Body.Close(); err != nil {
			cc.logger.Warn("failed to close response body", zap.Error(err))
		}
	}()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode >= 400 {
			cc.logger.Error("couchdb", zap.Error(err), zap.String("status_code", strconv.Itoa(resp.StatusCode)))
		}
		return nil, fmt.Errorf("request GET %s failed - %q", req.URL.String(), resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body %w", err)
	}

	return body, nil
}

// Nodes contains a list of all known connected node names.
type Nodes struct {
	AllNodes []string `json:"all_nodes"`
}

// GetNodeNames gets all known connected nodes names.
func (cc *couchDBClient) GetNodeNames() ([]string, error) {
	body, err := cc.Get(nodeNamesPath)
	if err != nil {
		return nil, err
	}

	var nodes Nodes
	err = json.Unmarshal(body, &nodes)
	if err != nil {
		return nil, err
	}

	return nodes.AllNodes, nil
}

// GetStats gets couchdb stats at a specific node name endpoint.
func (cc *couchDBClient) GetStats(nodeName string) (map[string]interface{}, error) {
	path := fmt.Sprintf("/_node/%s/_stats/couchdb", nodeName)
	body, err := cc.Get(path)
	if err != nil {
		return nil, err
	}

	var stats map[string]interface{}
	err = json.Unmarshal(body, &stats)
	if err != nil {
		return nil, err
	}

	return stats, nil
}

func (cc *couchDBClient) buildReq(path string) (*http.Request, error) {
	url := cc.cfg.Endpoint + path
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(cc.cfg.Username, cc.cfg.Password)
	return req, nil
}
