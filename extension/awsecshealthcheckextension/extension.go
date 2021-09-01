// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awsecshealthcheckextension

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/jaegertracing/jaeger/pkg/healthcheck"
	"go.uber.org/zap"

	"go.opencensus.io/stats/view"
	"go.opentelemetry.io/collector/component"
)

type ecsHealthCheckExtension struct {
	config   Config
	logger   *zap.Logger
	state    *healthcheck.HealthCheck
	server   http.Server
	interval time.Duration
	stopCh   chan struct{}
	exporter *ecsHealthCheckExporter
}

// initExporter function could register the customized exporter
func (hc *ecsHealthCheckExtension) initExporter() error {
	hc.exporter = newECSHealthCheckExporter()
	view.RegisterExporter(hc.exporter)
	return nil
}

func (hc *ecsHealthCheckExtension) Start(_ context.Context, host component.Host) error {
	hc.logger.Info("Starting ECS health check extension", zap.Any("config", hc.config))

	// Initialize listener
	var (
		ln  net.Listener
		err error
	)

	err = hc.config.Validate()
	if err != nil {
		return err
	}

	err = hc.initExporter()
	if err != nil {
		return err
	}

	ln, err = hc.config.TCPAddr.Listen()
	if err != nil {
		return err
	}

	hc.interval, err = time.ParseDuration(hc.config.Interval)
	if err != nil {
		return err
	}

	ticker := time.NewTicker(1 * time.Minute)

	hc.server.Handler = hc.handler()
	hc.stopCh = make(chan struct{})
	go func() {
		defer close(hc.stopCh)

		select {
		case <-ticker.C:
			hc.exporter.rotate(hc.interval)
		default:
		}

		if err := hc.server.Serve(ln); err != http.ErrServerClosed && err != nil {
			host.ReportFatalError(err)
		}

	}()
	return nil
}

func (hc *ecsHealthCheckExtension) check() bool {
	hc.exporter.mu.Lock()
	defer hc.exporter.mu.Unlock()

	return hc.config.ExporterErrorLimit >= len(hc.exporter.exporterErrorQueue)
}

func (hc *ecsHealthCheckExtension) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if hc.check() {
			w.WriteHeader(200)
		} else {
			w.WriteHeader(500)
		}
	})
}

func (hc *ecsHealthCheckExtension) Shutdown(context.Context) error {
	err := hc.server.Close()
	if hc.stopCh != nil {
		<-hc.stopCh
	}
	return err
}

func newServer(config Config, logger *zap.Logger) *ecsHealthCheckExtension {
	hc := &ecsHealthCheckExtension{
		config: config,
		logger: logger,
		state:  healthcheck.New(),
		server: http.Server{},
	}

	hc.state.SetLogger(logger)

	return hc
}
