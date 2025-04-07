// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package http // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckv2extension/internal/http"

import (
	"net/http"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckv2extension/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/status"
)

func (s *Server) statusHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pipeline := r.URL.Query().Get("pipeline")
		verboseParam := r.URL.Query().Has("verbose") && r.URL.Query().Get("verbose") != "false"
		verbose := status.Verbosity(verboseParam)
		st, ok := s.aggregator.AggregateStatus(status.Scope(pipeline), verbose)

		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		for pipeline, pipelineStatus := range s.aggregator.FindNotOk(verbose) {
			s.telemetry.Logger.Warn("pipeline not ok", common.HealthLogFields(pipeline, pipelineStatus, verbose)...)
		}

		if err := s.responder.respond(st, w); err != nil {
			s.telemetry.Logger.Warn(err.Error())
		}
	})
}

func (s *Server) configHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		conf := s.colconf.Load()

		if conf == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(conf.([]byte)); err != nil {
			s.telemetry.Logger.Warn(err.Error())
		}
	})
}
