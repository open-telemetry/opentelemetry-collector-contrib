// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package healthcheckextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension"

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/extension/extensioncapabilities"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension/internal/healthcheck"
)

type healthCheckExtension struct {
	config   Config
	logger   *zap.Logger
	state    *healthcheck.HealthCheck
	server   *http.Server
	stopCh   chan struct{}
	settings component.TelemetrySettings
}

var _ extensioncapabilities.PipelineWatcher = (*healthCheckExtension)(nil)

func (hc *healthCheckExtension) Start(ctx context.Context, host component.Host) error {
	hc.logger.Info("Starting health_check extension", zap.Any("config", hc.config))
	ln, err := hc.config.ToListener(ctx)
	if err != nil {
		return fmt.Errorf("failed to bind to address %s: %w", hc.config.Endpoint, err)
	}

	hc.server, err = hc.config.ToServer(ctx, host, hc.settings, nil)
	if err != nil {
		return err
	}

	// Mount HC handler
	mux := http.NewServeMux()
	mux.Handle(hc.config.Path, hc.baseHandler())
	hc.server.Handler = mux
	hc.stopCh = make(chan struct{})
	go func() {
		defer close(hc.stopCh)

		// The listener ownership goes to the server.
		if err = hc.server.Serve(ln); !errors.Is(err, http.ErrServerClosed) && err != nil {
			componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(err))
		}
	}()

	return nil
}

// base handler function
func (hc *healthCheckExtension) baseHandler() http.Handler {
	if hc.config.ResponseBody != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if hc.state.Get() == healthcheck.Ready {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(hc.config.ResponseBody.Healthy))
			} else {
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte(hc.config.ResponseBody.Unhealthy))
			}
		})
	}
	return hc.state.Handler()
}

func (hc *healthCheckExtension) Shutdown(context.Context) error {
	if hc.server == nil {
		return nil
	}
	err := hc.server.Close()
	if hc.stopCh != nil {
		<-hc.stopCh
	}
	return err
}

func (hc *healthCheckExtension) Ready() error {
	hc.state.Set(healthcheck.Ready)
	return nil
}

func (hc *healthCheckExtension) NotReady() error {
	hc.state.Set(healthcheck.Unavailable)
	return nil
}

func newServer(config Config, settings component.TelemetrySettings) *healthCheckExtension {
	hc := &healthCheckExtension{
		config:   config,
		logger:   settings.Logger,
		state:    healthcheck.New(),
		settings: settings,
	}

	hc.state.SetLogger(settings.Logger)

	return hc
}
