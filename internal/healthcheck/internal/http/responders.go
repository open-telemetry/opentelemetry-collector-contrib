// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package http // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/healthcheck/internal/http"

import (
	"encoding/json"
	"net/http"
	"time"

	"go.opentelemetry.io/collector/component/componentstatus"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/healthcheck/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/status"
)

var responseCodes = map[componentstatus.Status]int{
	componentstatus.StatusNone:             http.StatusServiceUnavailable,
	componentstatus.StatusStarting:         http.StatusServiceUnavailable,
	componentstatus.StatusOK:               http.StatusOK,
	componentstatus.StatusRecoverableError: http.StatusOK,
	componentstatus.StatusPermanentError:   http.StatusOK,
	componentstatus.StatusFatalError:       http.StatusInternalServerError,
	componentstatus.StatusStopping:         http.StatusServiceUnavailable,
	componentstatus.StatusStopped:          http.StatusServiceUnavailable,
}

type serializationErr struct {
	ErrorMessage string `json:"error_message"`
}

type responder interface {
	respond(*status.AggregateStatus, http.ResponseWriter, bool) error
}

type responderFunc func(*status.AggregateStatus, http.ResponseWriter, bool) error

func (f responderFunc) respond(st *status.AggregateStatus, w http.ResponseWriter, verbose bool) error {
	return f(st, w, verbose)
}

func respondWithJSON(code int, content any, w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	body, mErr := json.Marshal(content)
	if mErr != nil {
		body, _ = json.Marshal(&serializationErr{ErrorMessage: mErr.Error()})
	}
	_, wErr := w.Write(body)
	return wErr
}

func defaultResponder(startTimestamp *time.Time, includeAttributes bool) responderFunc {
	return func(st *status.AggregateStatus, w http.ResponseWriter, verbose bool) error {
		code := responseCodes[st.Status()]
		sst := toSerializableStatus(st, &serializationOptions{
			includeStartTime:  true,
			startTimestamp:    startTimestamp,
			includeAttributes: includeAttributes && verbose,
		})
		return respondWithJSON(code, sst, w)
	}
}

func componentHealthResponder(
	startTimestamp *time.Time,
	config *common.ComponentHealthConfig,
	includeAttributes bool,
) responderFunc {
	healthyFunc := func(now *time.Time) func(status.Event) bool {
		return func(ev status.Event) bool {
			if ev.Status() == componentstatus.StatusPermanentError {
				return !config.IncludePermanent
			}

			if ev.Status() == componentstatus.StatusRecoverableError && config.IncludeRecoverable {
				return now.Before(ev.Timestamp().Add(config.RecoveryDuration))
			}

			return ev.Status() != componentstatus.StatusFatalError
		}
	}
	return func(st *status.AggregateStatus, w http.ResponseWriter, verbose bool) error {
		now := time.Now()
		sst := toSerializableStatus(
			st,
			&serializationOptions{
				includeStartTime:  true,
				startTimestamp:    startTimestamp,
				healthyFunc:       healthyFunc(&now),
				includeAttributes: includeAttributes && verbose,
			},
		)

		code := responseCodes[st.Status()]
		if !sst.Healthy {
			code = http.StatusInternalServerError
		}

		return respondWithJSON(code, sst, w)
	}
}

// Below are responders ported from the original healthcheck extension. We will
// keep them for backwards compatibility, but eventually deprecate and remove
// them.

// legacyResponseCodes match the current response code mapping with the exception
// of FatalError, which maps to 503 instead of 500.
var legacyResponseCodes = map[componentstatus.Status]int{
	componentstatus.StatusNone:             http.StatusServiceUnavailable,
	componentstatus.StatusStarting:         http.StatusServiceUnavailable,
	componentstatus.StatusOK:               http.StatusOK,
	componentstatus.StatusRecoverableError: http.StatusOK,
	componentstatus.StatusPermanentError:   http.StatusOK,
	componentstatus.StatusFatalError:       http.StatusServiceUnavailable,
	componentstatus.StatusStopping:         http.StatusServiceUnavailable,
	componentstatus.StatusStopped:          http.StatusServiceUnavailable,
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

	return func(st *status.AggregateStatus, w http.ResponseWriter, _ bool) error {
		code := legacyResponseCodes[st.Status()]
		resp := healthCheckResponse{
			StatusMsg: codeToMsgMap[code],
		}
		if code == http.StatusOK {
			resp.UpSince = *startTimestamp
			resp.Uptime = time.Since(*startTimestamp).String()
		}
		return respondWithJSON(code, resp, w)
	}
}

func legacyCustomResponder(config *ResponseBodyConfig) responderFunc {
	codeToMsgMap := map[int][]byte{
		http.StatusOK:                 []byte(config.Healthy),
		http.StatusServiceUnavailable: []byte(config.Unhealthy),
	}
	return func(st *status.AggregateStatus, w http.ResponseWriter, _ bool) error {
		code := legacyResponseCodes[st.Status()]
		w.WriteHeader(code)
		_, err := w.Write(codeToMsgMap[code])
		return err
	}
}
