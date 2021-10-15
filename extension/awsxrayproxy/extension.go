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

package awsxrayproxy

import (
	"context"
	"net/http"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy"
)

type xrayProxy struct {
	logger *zap.Logger
	config *Config
	server proxy.Server
}

var _ component.Extension = (*xrayProxy)(nil)

func (x xrayProxy) Start(ctx context.Context, host component.Host) error {
	go func() {
		if err := x.server.ListenAndServe(); err != nil {
			if err != http.ErrServerClosed {
				host.ReportFatalError(err)
			}
		}
	}()
	x.logger.Info("X-Ray proxy server started on " + x.config.ProxyConfig.Endpoint)
	return nil
}

func (x xrayProxy) Shutdown(ctx context.Context) error {
	return x.server.Shutdown(ctx)
}

func newXrayProxy(config *Config, logger *zap.Logger) (component.Extension, error) {
	srv, err := proxy.NewServer(&config.ProxyConfig, logger)

	if err != nil {
		return nil, err
	}

	p := &xrayProxy{
		config: config,
		logger: logger,
		server: srv,
	}

	return p, nil
}
