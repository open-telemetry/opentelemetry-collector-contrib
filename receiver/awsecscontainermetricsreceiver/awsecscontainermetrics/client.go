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
	"io/ioutil"
	"net/http"

	"go.uber.org/zap"
)

type Client interface {
	Get(path string) ([]byte, error)
}

func NewClientProvider(endpoint string, logger *zap.Logger) (ClientProvider, error) {
	return &defaultClientProvider{
		endpoint: endpoint,
		logger:   logger,
	}, nil
}

type ClientProvider interface {
	BuildClient() (Client, error)
}

type defaultClientProvider struct {
	endpoint string
	logger   *zap.Logger
}

func (dcp *defaultClientProvider) BuildClient() (Client, error) {
	return defaultClient(
		dcp.endpoint,
		dcp.logger,
	)
}

func defaultClient(
	endpoint string,
	logger *zap.Logger,
) (*clientImpl, error) {
	return &clientImpl{
		baseURL:    "https://" + endpoint,
		httpClient: http.Client{},
		logger:     logger,
	}, nil
}

// clientImpl

var _ Client = (*clientImpl)(nil)

type clientImpl struct {
	baseURL    string
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
			c.logger.Warn("failed to close response body", zap.Error(closeErr))
		}
	}()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func (c *clientImpl) buildReq(path string) (*http.Request, error) {
	url := c.baseURL + path
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}
