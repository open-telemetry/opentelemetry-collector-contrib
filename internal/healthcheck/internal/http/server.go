// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package http // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/healthcheck/internal/http"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/extension/extensioncapabilities"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/healthcheck/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/status"
)

type Server struct {
	telemetry      component.TelemetrySettings
	httpConfig     confighttp.ServerConfig
	httpServer     *http.Server
	mux            *http.ServeMux
	responder      responder
	colconf        atomic.Value
	aggregator     *status.Aggregator
	startTimestamp time.Time
	doneWg         sync.WaitGroup
	doneCh         chan struct{}
	doneOnce       sync.Once
}

var (
	_ component.Component                 = (*Server)(nil)
	_ extensioncapabilities.ConfigWatcher = (*Server)(nil)
)

func NewServer(
	config *Config,
	legacyConfig LegacyConfig,
	componentHealthConfig *common.ComponentHealthConfig,
	telemetry component.TelemetrySettings,
	aggregator *status.Aggregator,
) *Server {
	now := time.Now()
	srv := &Server{
		telemetry:  telemetry,
		mux:        http.NewServeMux(),
		aggregator: aggregator,
		doneCh:     make(chan struct{}),
	}

	if legacyConfig.UseV2 {
		srv.httpConfig = config.ServerConfig
		if componentHealthConfig != nil {
			srv.responder = componentHealthResponder(&now, componentHealthConfig)
		} else {
			srv.responder = defaultResponder(&now)
		}
		if config.Status.Enabled {
			srv.mux.Handle(config.Status.Path, srv.statusHandler())
		}
		if config.Config.Enabled {
			srv.mux.Handle(config.Config.Path, srv.configHandler())
		}
	} else {
		srv.httpConfig = legacyConfig.ServerConfig
		if legacyConfig.ResponseBody != nil {
			srv.responder = legacyCustomResponder(legacyConfig.ResponseBody)
		} else {
			srv.responder = legacyDefaultResponder(&now)
		}
		srv.mux.Handle(legacyConfig.Path, srv.statusHandler())
	}

	return srv
}

// Start implements the component.Component interface.
func (s *Server) Start(ctx context.Context, host component.Host) error {
	var err error
	s.startTimestamp = time.Now()

	s.httpServer, err = s.httpConfig.ToServer(ctx, host.GetExtensions(), s.telemetry, s.mux)
	if err != nil {
		return err
	}

	ln, err := s.httpConfig.ToListener(ctx)
	if err != nil {
		// Server never started, ensure doneCh is closed so shutdown doesn't block
		s.doneOnce.Do(func() { close(s.doneCh) })
		return fmt.Errorf("failed to bind to address %s: %w", s.httpConfig.NetAddr.Endpoint, err)
	}

	s.doneWg.Go(func() {
		defer s.doneOnce.Do(func() { close(s.doneCh) })

		if err = s.httpServer.Serve(ln); !errors.Is(err, http.ErrServerClosed) && err != nil {
			componentstatus.ReportStatus(host, componentstatus.NewPermanentErrorEvent(err))
		}
	})

	return nil
}

// Shutdown implements the component.Component interface.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}
	s.httpServer.Close()
	select {
	case <-s.doneCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// NotifyConfig implements the extension.ConfigWatcher interface.
func (s *Server) NotifyConfig(_ context.Context, conf *confmap.Conf) error {
	confBytes, err := json.Marshal(conf.ToStringMap())
	if err != nil {
		s.telemetry.Logger.Warn("could not marshal config", zap.Error(err))
		return err
	}
	s.colconf.Store(confBytes)
	return nil
}
