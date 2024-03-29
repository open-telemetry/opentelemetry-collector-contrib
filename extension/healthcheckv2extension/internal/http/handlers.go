// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package http // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckv2extension/internal/http"

import (
	"net/http"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckv2extension/internal/status"
)

func (s *Server) statusHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pipeline := r.URL.Query().Get("pipeline")
		verbose := r.URL.Query().Has("verbose")
		st, ok := s.aggregator.AggregateStatus(status.Scope(pipeline), status.Verbosity(verbose))

		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		s.responder.respond(st, w)
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
		_, _ = w.Write(conf.([]byte))
	})
}
