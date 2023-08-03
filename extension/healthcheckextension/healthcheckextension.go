// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package healthcheckextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension"

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/jaegertracing/jaeger/pkg/healthcheck"
	"go.opencensus.io/stats/view"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"
)

type healthCheckExtension struct {
	config   Config
	logger   *zap.Logger
	state    *healthcheck.HealthCheck
	server   *http.Server
	stopCh   chan struct{}
	exporter *healthCheckExporter
	settings component.TelemetrySettings
}

var _ extension.PipelineWatcher = (*healthCheckExtension)(nil)

func (hc *healthCheckExtension) Start(_ context.Context, host component.Host) error {

	hc.logger.Info("Starting health_check extension", zap.Any("config", hc.config))
	ln, err := hc.config.ToListener()
	if err != nil {
		return fmt.Errorf("failed to bind to address %s: %w", hc.config.Endpoint, err)
	}

	hc.server, err = hc.config.ToServer(host, hc.settings, nil)
	if err != nil {
		return err
	}

	if !hc.config.CheckCollectorPipeline.Enabled {
		// Mount HC handler
		mux := http.NewServeMux()
		mux.Handle(hc.config.Path, hc.baseHandler())
		hc.server.Handler = mux
		hc.stopCh = make(chan struct{})
		go func() {
			defer close(hc.stopCh)

			// The listener ownership goes to the server.
			if err = hc.server.Serve(ln); !errors.Is(err, http.ErrServerClosed) && err != nil {
				host.ReportFatalError(err)
			}
		}()
	} else {
		// collector pipeline health check
		hc.exporter = newHealthCheckExporter()
		view.RegisterExporter(hc.exporter)

		interval, err := time.ParseDuration(hc.config.CheckCollectorPipeline.Interval)
		if err != nil {
			return err
		}

		// ticker used by collector pipeline health check for rotation
		ticker := time.NewTicker(time.Second)

		mux := http.NewServeMux()
		mux.Handle(hc.config.Path, hc.checkCollectorPipelineHandler())
		hc.server.Handler = mux
		hc.stopCh = make(chan struct{})
		go func() {
			defer close(hc.stopCh)
			defer view.UnregisterExporter(hc.exporter)

			go func() {
				for {
					select {
					case <-ticker.C:
						hc.exporter.rotate(interval)
					case <-hc.stopCh:
						return
					}
				}
			}()

			if errHTTP := hc.server.Serve(ln); !errors.Is(errHTTP, http.ErrServerClosed) && errHTTP != nil {
				host.ReportFatalError(errHTTP)
			}

		}()
	}

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

// new handler function used for check collector pipeline
func (hc *healthCheckExtension) checkCollectorPipelineHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if hc.check() && hc.state.Get() == healthcheck.Ready {
			w.WriteHeader(http.StatusOK)
			if hc.config.ResponseBody != nil {
				_, _ = w.Write([]byte(hc.config.ResponseBody.Healthy))
			}
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			if hc.config.ResponseBody != nil {
				_, _ = w.Write([]byte(hc.config.ResponseBody.Unhealthy))
			}
		}
	})
}

func (hc *healthCheckExtension) check() bool {
	return hc.exporter.checkHealthStatus(hc.config.CheckCollectorPipeline.ExporterFailureThreshold)
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
