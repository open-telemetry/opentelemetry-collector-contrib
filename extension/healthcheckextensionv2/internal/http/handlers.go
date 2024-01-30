// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package http // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2/internal/http"

import (
	"encoding/json"
	"net/http"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2/internal/status"
)

func (s *Server) statusHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.isReady() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		pipeline := r.URL.Query().Get("pipeline")
		st, ok := s.aggregator.AggregateStatus(
			status.Scope(pipeline),
			status.Detail(s.settings.Status.Detailed),
		)

		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		sst := toSerializableStatus(st, s.startTimestamp, s.recoveryDuration)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(s.toHTTPStatus(sst))

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

var responseCodes = map[component.Status]int{
	component.StatusNone:             http.StatusServiceUnavailable,
	component.StatusStarting:         http.StatusServiceUnavailable,
	component.StatusOK:               http.StatusOK,
	component.StatusRecoverableError: http.StatusOK,
	component.StatusPermanentError:   http.StatusInternalServerError,
	component.StatusFatalError:       http.StatusInternalServerError,
	component.StatusStopping:         http.StatusServiceUnavailable,
	component.StatusStopped:          http.StatusServiceUnavailable,
}

func (s *Server) toHTTPStatus(sst *serializableStatus) int {
	if sst.Status() == component.StatusRecoverableError && !sst.Healthy {
		return http.StatusInternalServerError
	}
	return responseCodes[sst.Status()]
}
