// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsproxy // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/awsproxy"

import (
	"context"
	"errors"
	"net/http"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy"
)

type xrayProxy struct {
	logger *zap.Logger
	config *Config
	server proxy.Server
}

var _ extension.Extension = (*xrayProxy)(nil)

func (x xrayProxy) Start(_ context.Context, host component.Host) error {
	go func() {
		if err := x.server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) && err != nil {
			host.ReportFatalError(err)
		}
	}()
	x.logger.Info("X-Ray proxy server started on " + x.config.ProxyConfig.Endpoint)
	return nil
}

func (x xrayProxy) Shutdown(ctx context.Context) error {
	return x.server.Shutdown(ctx)
}

func newXrayProxy(config *Config, logger *zap.Logger) (extension.Extension, error) {
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
