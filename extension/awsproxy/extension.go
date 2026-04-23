// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsproxy // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/awsproxy"

import (
	"context"
	"errors"
	"net/http"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy"
)

type xrayProxy struct {
	logger   *zap.Logger
	config   *Config
	server   proxy.Server
	settings component.TelemetrySettings
}

var _ extension.Extension = (*xrayProxy)(nil)

func (x *xrayProxy) Start(_ context.Context, host component.Host) error {
	srv, err := proxy.NewServer(&x.config.ProxyConfig, x.settings.Logger)
	if err != nil {
		return err
	}
	x.server = srv
	go func() {
		if err := x.server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) && err != nil {
			componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(err))
		}
	}()
	x.logger.Info("X-Ray proxy server started on " + x.config.ProxyConfig.Endpoint)
	return nil
}

func (x *xrayProxy) Shutdown(ctx context.Context) error {
	if x.server != nil {
		return x.server.Shutdown(ctx)
	}
	return nil
}

func newXrayProxy(config *Config, telemetrySettings component.TelemetrySettings) (extension.Extension, error) {
	p := &xrayProxy{
		config:   config,
		logger:   telemetrySettings.Logger,
		settings: telemetrySettings,
	}

	return p, nil
}
