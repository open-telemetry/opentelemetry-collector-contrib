// Copyright The OpenTelemetry Authors
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

package httpforwarder

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
)

type httpForwarder struct {
	listenAt   string
	forwardTo  *url.URL
	headers    map[string]string
	httpClient *http.Client
	server     *http.Server
	logger     *zap.Logger
}

var _ component.ServiceExtension = (*httpForwarder)(nil)

func (h httpForwarder) Start(_ context.Context, _ component.Host) error {
	// TODO: Start HTTP server
	return nil
}

func (h httpForwarder) Shutdown(_ context.Context) error {
	// TODO: Shutdown HTTP server
	return nil
}

func newHTTPForwarder(config *Config, logger *zap.Logger) (component.ServiceExtension, error) {
	if config.Upstream.Endpoint == "" {
		return nil, errors.New("'upstream.endpoint' config option cannot be empty")
	}

	var url, err = url.Parse(config.Upstream.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("enter a valid URL for 'upstream.endpoint': %w", err)
	}

	handler := http.NewServeMux()
	server := &http.Server{
		Addr:    config.Endpoint,
		Handler: handler,
	}

	httpClient, err := config.Upstream.ToClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP Client: %w", err)
	}

	h := &httpForwarder{
		listenAt:   config.Endpoint,
		forwardTo:  url,
		headers:    config.Upstream.Headers,
		httpClient: httpClient,
		server:     server,
		logger:     logger,
	}
	// TODO: Register handler method

	return h, nil
}
