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
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2/internal/status"
)

type Server struct {
	telemetry      component.TelemetrySettings
	httpSettings   confighttp.HTTPServerSettings
	httpServer     *http.Server
	mux            *http.ServeMux
	responder      responder
	colconf        atomic.Value
	aggregator     *status.Aggregator
	startTimestamp time.Time
	doneCh         chan struct{}
}

var _ component.Component = (*Server)(nil)
var _ extension.ConfigWatcher = (*Server)(nil)

func NewServer(
	settings *Settings,
	legacySettings LegacySettings,
	componentHealthSettings *common.ComponentHealthSettings,
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

	if legacySettings.UseV2Settings {
		srv.httpSettings = settings.HTTPServerSettings
		if componentHealthSettings != nil {
			srv.responder = componentHealthResponder(&now, componentHealthSettings)
		} else {
			srv.responder = defaultResponder(&now)
		}
		if settings.Status.Enabled {
			srv.mux.Handle(settings.Status.Path, srv.statusHandler())
		}
		if settings.Config.Enabled {
			srv.mux.Handle(settings.Config.Path, srv.configHandler())
		}
	} else {
		srv.httpSettings = legacySettings.HTTPServerSettings
		if legacySettings.ResponseBody != nil {
			srv.responder = legacyCustomResponder(legacySettings.ResponseBody)
		} else {
			srv.responder = legacyDefaultResponder(&now)
		}
		srv.mux.Handle(legacySettings.Path, srv.statusHandler())
	}

	return srv
}

// Start implements the component.Component interface.
func (s *Server) Start(_ context.Context, host component.Host) error {
	var err error
	s.startTimestamp = time.Now()

	s.httpServer, err = s.httpSettings.ToServer(host, s.telemetry, s.mux)
	if err != nil {
		return err
	}

	ln, err := s.httpSettings.ToListener()
	if err != nil {
		return fmt.Errorf("failed to bind to address %s: %w", s.httpSettings.Endpoint, err)
	}

	go func() {
		defer close(s.doneCh)
		if err = s.httpServer.Serve(ln); !errors.Is(err, http.ErrServerClosed) && err != nil {
			s.telemetry.ReportStatus(component.NewPermanentErrorEvent(err))
		}
	}()

	return nil
}

// Shutdown implements the component.Component interface.
func (s *Server) Shutdown(context.Context) error {
	if s.httpServer == nil {
		return nil
	}
	s.httpServer.Close()
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
