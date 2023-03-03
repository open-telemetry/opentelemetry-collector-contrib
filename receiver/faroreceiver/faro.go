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

package faroreceiver // import "github.com/grafana/opentelemetry-collector-components/components/receiver/faroreceiver"

import (
	"context"
	"errors"
	"net/http"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

type faroReceiver struct {
	cfg *Config
	set receiver.CreateSettings

	server       *http.Server
	startOnce    sync.Once
	shutdownOnce sync.Once
	shutdownWG   sync.WaitGroup

	logConsumer   consumer.Logs
	traceConsumer consumer.Traces

	logLogger   *zap.Logger
	traceLogger *zap.Logger

	// TODO
	// obsrepHTTP *obsreport.Receiver
}

// newFaroReceiver TODO...
func newFaroReceiver(cfg Config, set receiver.CreateSettings) *faroReceiver {
	return &faroReceiver{
		cfg: &cfg,
		set: set,
	}
}

func (r *faroReceiver) registerTraceConsumer(log *zap.Logger, tc consumer.Traces) error {
	if tc == nil {
		return component.ErrNilNextConsumer
	}
	r.traceLogger = log
	r.traceConsumer = tc
	return nil
}

func (r *faroReceiver) registerLogConsumer(log *zap.Logger, lc consumer.Logs) error {
	if lc == nil {
		return component.ErrNilNextConsumer
	}
	r.logLogger = log
	r.logConsumer = lc
	return nil
}

// Start will start the receiver http server
func (r *faroReceiver) Start(ctx context.Context, host component.Host) error {
	var err error
	r.startOnce.Do(func() {
		if r.cfg.HTTP != nil {
			if r.server, err = r.cfg.HTTP.ToServer(
				host,
				r.set.TelemetrySettings,
				&Handler{
					logConsumer:   r.logConsumer,
					traceConsumer: r.traceConsumer,
				},
			); err != nil {
				return
			}

			err = r.startHTTPServer(r.cfg.HTTP, host)
		}
	})
	return err
}

// Shutdown stops the receiver
func (r *faroReceiver) Shutdown(ctx context.Context) error {
	var err error
	r.shutdownOnce.Do(func() {
		r.set.Logger.Info("Stopping HTTP server")
		if r.server == nil {
			return
		}
		err = r.server.Shutdown(ctx)
		r.shutdownWG.Wait()
	})
	return err
}

func (r *faroReceiver) startHTTPServer(cfg *confighttp.HTTPServerSettings, host component.Host) error {
	r.set.Logger.Info("Starting HTTP server", zap.String("endpoint", cfg.Endpoint))
	hln, err := cfg.ToListener()
	if err != nil {
		return err
	}
	r.shutdownWG.Add(1)
	go func() {
		defer r.shutdownWG.Done()

		if errHTTP := r.server.Serve(hln); errHTTP != nil && !errors.Is(errHTTP, http.ErrServerClosed) {
			host.ReportFatalError(errHTTP)
		}
	}()
	return err
}
