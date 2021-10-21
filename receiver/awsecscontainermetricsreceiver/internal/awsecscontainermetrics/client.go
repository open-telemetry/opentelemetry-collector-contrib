// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awsecscontainermetrics

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"go.uber.org/zap"
)

// Client defines the rest client interface
type Client interface {
	Get(path string) ([]byte, error)
}

// NewClientProvider creates the default rest client provider
func NewClientProvider(endpoint url.URL, logger *zap.Logger) ClientProvider {
	return &defaultClientProvider{
		endpoint: endpoint,
		logger:   logger,
	}
}

// ClientProvider defines
type ClientProvider interface {
	BuildClient() Client
}

type defaultClientProvider struct {
	endpoint url.URL
	logger   *zap.Logger
}

func (dcp *defaultClientProvider) BuildClient() Client {
	return defaultClient(
		dcp.endpoint,
		dcp.logger,
	)
}

// TODO: Try using config.HTTPClientSettings
func defaultClient(
	endpoint url.URL,
	logger *zap.Logger,
) *clientImpl {
	tr := defaultTransport()
	return &clientImpl{
		baseURL:    endpoint,
		httpClient: http.Client{Transport: tr},
		logger:     logger,
	}
}

func defaultTransport() *http.Transport {
	return http.DefaultTransport.(*http.Transport).Clone()
}

var _ Client = (*clientImpl)(nil)

type clientImpl struct {
	baseURL    url.URL
	httpClient http.Client
	logger     *zap.Logger
}

func (c *clientImpl) Get(path string) ([]byte, error) {
	req, err := c.buildReq(path)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		closeErr := resp.Body.Close()
		if closeErr != nil {
			c.logger.Warn("Failed to close response body", zap.Error(closeErr))
		}
	}()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request GET %s failed - %q", req.URL.String(), resp.Status)
	}
	return body, nil
}

func (c *clientImpl) buildReq(path string) (*http.Request, error) {
	url := c.baseURL.String() + path
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}
