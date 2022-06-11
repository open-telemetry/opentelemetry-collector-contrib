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

package jaegerremotesampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/jaegerremotesampling"

import (
	"context"
	"fmt"

	grpcStore "github.com/jaegertracing/jaeger/cmd/agent/app/configmanager/grpc"
	"github.com/jaegertracing/jaeger/cmd/collector/app/sampling/strategystore"
	"github.com/jaegertracing/jaeger/plugin/sampling/strategystore/static"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/jaegerremotesampling/internal"
)

var _ component.Extension = (*jrsExtension)(nil)

type jrsExtension struct {
	cfg       *Config
	telemetry component.TelemetrySettings

	httpServer    component.Component
	samplingStore strategystore.StrategyStore

	closers []func() error
}

func newExtension(cfg *Config, telemetry component.TelemetrySettings) *jrsExtension {
	jrse := &jrsExtension{
		cfg:       cfg,
		telemetry: telemetry,
	}
	return jrse
}

func (jrse *jrsExtension) Start(ctx context.Context, host component.Host) error {
	// the config validation will take care of ensuring we have one and only one of the following about the
	// source of the sampling config:
	// - remote (gRPC)
	// - local file
	// we can then use a simplified logic here to assign the appropriate store
	if jrse.cfg.Source.File != "" {
		opts := static.Options{
			StrategiesFile: jrse.cfg.Source.File,
			ReloadInterval: jrse.cfg.Source.ReloadInterval,
		}
		ss, err := static.NewStrategyStore(opts, jrse.telemetry.Logger)
		if err != nil {
			return fmt.Errorf("failed to create the local file strategy store: %v", err)
		}

		// there's a Close function on the concrete type, which is not visible to us...
		// how can we close it then?
		jrse.samplingStore = ss
	}

	if jrse.cfg.Source.Remote != nil {
		opts, err := jrse.cfg.Source.Remote.ToDialOptions(host, jrse.telemetry)
		if err != nil {
			return fmt.Errorf("error while setting up the remote sampling source: %v", err)
		}
		conn, err := grpc.Dial(jrse.cfg.Source.Remote.Endpoint, opts...)
		if err != nil {
			return fmt.Errorf("error while connecting to the remote sampling source: %v", err)
		}

		jrse.samplingStore = grpcStore.NewConfigManager(conn)
		jrse.closers = append(jrse.closers, func() error {
			return conn.Close()
		})
	}

	if jrse.cfg.HTTPServerSettings != nil {
		httpServer, err := internal.NewHTTP(jrse.telemetry, *jrse.cfg.HTTPServerSettings, jrse.samplingStore)
		if err != nil {
			return fmt.Errorf("error while creating the HTTP server: %v", err)
		}
		jrse.httpServer = httpServer
	}

	// then we start our own server interfaces, starting with the HTTP one
	err := jrse.httpServer.Start(ctx, host)
	if err != nil {
		return fmt.Errorf("error while starting the HTTP server: %v", err)
	}

	return nil
}

func (jrse *jrsExtension) Shutdown(ctx context.Context) error {
	// we probably don't want to break whenever an error occurs, we want to continue and close the other resources
	if err := jrse.httpServer.Shutdown(ctx); err != nil {
		jrse.telemetry.Logger.Error("error while shutting down the HTTP server", zap.Error(err))
	}

	for _, closer := range jrse.closers {
		if err := closer(); err != nil {
			jrse.telemetry.Logger.Error("error while shutting down the sampling store", zap.Error(err))
		}
	}

	return nil
}
