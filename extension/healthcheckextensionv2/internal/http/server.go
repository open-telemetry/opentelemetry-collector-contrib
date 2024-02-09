// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package http // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2/internal/http"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2/internal/status"
)

type Server struct {
	telemetry      component.TelemetrySettings
	settings       *Settings
	strategy       healthStrategy
	mux            *http.ServeMux
	serverHTTP     *http.Server
	colconf        atomic.Value
	aggregator     *status.Aggregator
	startTimestamp time.Time
	doneCh         chan struct{}
}

var _ component.Component = (*Server)(nil)
var _ extension.ConfigWatcher = (*Server)(nil)

func NewServer(
	settings *Settings,
	componentHealthSettings *common.ComponentHealthSettings,
	telemetry component.TelemetrySettings,
	aggregator *status.Aggregator,
) *Server {
	now := time.Now()
	srv := &Server{
		telemetry:  telemetry,
		settings:   settings,
		strategy:   &defaultHealthStrategy{startTimestamp: &now},
		aggregator: aggregator,
		doneCh:     make(chan struct{}),
	}

	if componentHealthSettings != nil {
		srv.strategy = &componentHealthStrategy{
			settings:       componentHealthSettings,
			startTimestamp: &now,
		}
	}

	srv.mux = http.NewServeMux()
	if settings.Status.Enabled {
		srv.mux.Handle(settings.Status.Path, srv.statusHandler())
	}
	if settings.Config.Enabled {
		srv.mux.Handle(settings.Config.Path, srv.configHandler())
	}

	return srv
}

// Start implements the component.Component interface.
func (s *Server) Start(_ context.Context, host component.Host) error {
	var err error
	s.startTimestamp = time.Now()

	s.serverHTTP, err = s.settings.ToServer(host, s.telemetry, s.mux)
	if err != nil {
		return err
	}

	ln, err := s.settings.ToListener()
	if err != nil {
		return fmt.Errorf("failed to bind to address %s: %w", s.settings.Endpoint, err)
	}

	go func() {
		defer close(s.doneCh)
		if err = s.serverHTTP.Serve(ln); !errors.Is(err, http.ErrServerClosed) && err != nil {
			s.telemetry.ReportStatus(component.NewPermanentErrorEvent(err))
		}
	}()

	return nil
}

// Shutdown implements the component.Component interface.
func (s *Server) Shutdown(context.Context) error {
	if s.serverHTTP == nil {
		return nil
	}
	s.serverHTTP.Close()
	<-s.doneCh
	return nil
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
