// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package http // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckv2extension/internal/http"

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckv2extension/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckv2extension/internal/status"
)

var responseCodes = map[component.Status]int{
	component.StatusNone:             http.StatusServiceUnavailable,
	component.StatusStarting:         http.StatusServiceUnavailable,
	component.StatusOK:               http.StatusOK,
	component.StatusRecoverableError: http.StatusOK,
	component.StatusPermanentError:   http.StatusOK,
	component.StatusFatalError:       http.StatusInternalServerError,
	component.StatusStopping:         http.StatusServiceUnavailable,
	component.StatusStopped:          http.StatusServiceUnavailable,
}

type responder interface {
	respond(*status.AggregateStatus, http.ResponseWriter)
}

type responderFunc func(*status.AggregateStatus, http.ResponseWriter)

func (f responderFunc) respond(st *status.AggregateStatus, w http.ResponseWriter) {
	f(st, w)
}

func respondWithJSON(code int, content any, w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	body, _ := json.Marshal(content)
	_, _ = w.Write(body)
}

func defaultResponder(startTimestamp *time.Time) responderFunc {
	return func(st *status.AggregateStatus, w http.ResponseWriter) {
		code := responseCodes[st.Status()]
		sst := toSerializableStatus(st, &serializationOptions{
			includeStartTime: true,
			startTimestamp:   startTimestamp,
		})
		respondWithJSON(code, sst, w)
	}
}

func componentHealthResponder(
	startTimestamp *time.Time,
	config *common.ComponentHealthConfig,
) responderFunc {
	healthyFunc := func(now *time.Time) func(status.Event) bool {
		return func(ev status.Event) bool {
			if ev.Status() == component.StatusPermanentError {
				return !config.IncludePermanent
			}

			if ev.Status() == component.StatusRecoverableError && config.IncludeRecoverable {
				return now.Before(ev.Timestamp().Add(config.RecoveryDuration))
			}

			return ev.Status() != component.StatusFatalError
		}
	}
	return func(st *status.AggregateStatus, w http.ResponseWriter) {
		now := time.Now()
		sst := toSerializableStatus(
			st,
			&serializationOptions{
				includeStartTime: true,
				startTimestamp:   startTimestamp,
				healthyFunc:      healthyFunc(&now),
			},
		)

		code := responseCodes[st.Status()]
		if !sst.Healthy {
			code = http.StatusInternalServerError
		}

		respondWithJSON(code, sst, w)
	}
}

// Below are responders ported from the original healthcheck extension. We will
// keep them for backwards compatibility, but eventually deprecate and remove
// them.

// legacyResponseCodes match the current response code mapping with the exception
// of FatalError, which maps to 503 instead of 500.
var legacyResponseCodes = map[component.Status]int{
	component.StatusNone:             http.StatusServiceUnavailable,
	component.StatusStarting:         http.StatusServiceUnavailable,
	component.StatusOK:               http.StatusOK,
	component.StatusRecoverableError: http.StatusOK,
	component.StatusPermanentError:   http.StatusOK,
	component.StatusFatalError:       http.StatusServiceUnavailable,
	component.StatusStopping:         http.StatusServiceUnavailable,
	component.StatusStopped:          http.StatusServiceUnavailable,
}

func legacyDefaultResponder(startTimestamp *time.Time) responderFunc {
	type healthCheckResponse struct {
		StatusMsg string    `json:"status"`
		UpSince   time.Time `json:"upSince"`
		Uptime    string    `json:"uptime"`
	}

	codeToMsgMap := map[int]string{
		http.StatusOK:                 "Server available",
		http.StatusServiceUnavailable: "Server not available",
	}

	return func(st *status.AggregateStatus, w http.ResponseWriter) {
		code := legacyResponseCodes[st.Status()]
		resp := healthCheckResponse{
			StatusMsg: codeToMsgMap[code],
		}
		if code == http.StatusOK {
			resp.UpSince = *startTimestamp
			resp.Uptime = fmt.Sprintf("%v", time.Since(*startTimestamp))
		}
		respondWithJSON(code, resp, w)
	}
}

func legacyCustomResponder(config *ResponseBodyConfig) responderFunc {
	codeToMsgMap := map[int][]byte{
		http.StatusOK:                 []byte(config.Healthy),
		http.StatusServiceUnavailable: []byte(config.Unhealthy),
	}
	return func(st *status.AggregateStatus, w http.ResponseWriter) {
		code := legacyResponseCodes[st.Status()]
		w.WriteHeader(code)
		_, _ = w.Write(codeToMsgMap[code])
	}
}
