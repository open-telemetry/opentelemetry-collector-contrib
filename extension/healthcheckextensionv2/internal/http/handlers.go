// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package http // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2/internal/http"

import (
	"encoding/json"
	"net/http"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2/internal/status"
)

func (s *Server) statusHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.isReady() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		pipeline := r.URL.Query().Get("pipeline")
		verbose := r.URL.Query().Has("verbose")
		st, ok := s.aggregator.AggregateStatus(status.Scope(pipeline))

		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		code, sst := s.strategy.toResponse(st, verbose)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)

		body, _ := json.Marshal(sst)
		_, _ = w.Write(body)
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
