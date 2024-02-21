// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package http // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2/internal/http"

import (
	"net/http"
	"time"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2/internal/status"
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

type healthResponder interface {
	response(*status.AggregateStatus) (int, *serializableStatus)
}

type responseFunc func(*status.AggregateStatus) (int, *serializableStatus)

func (f responseFunc) response(st *status.AggregateStatus) (int, *serializableStatus) {
	return f(st)
}

func defaultHealthResponder(startTimestamp *time.Time) responseFunc {
	return func(st *status.AggregateStatus) (int, *serializableStatus) {
		return responseCodes[st.Status()], toSerializableStatus(st, &serializationOptions{
			includeStartTime: true,
			startTimestamp:   startTimestamp,
		})
	}
}

func componentHealthResponder(
	startTimestamp *time.Time,
	settings *common.ComponentHealthSettings,
) responseFunc {
	healthyFunc := func(now *time.Time) func(status.Event) bool {
		return func(ev status.Event) bool {
			if ev.Status() == component.StatusPermanentError {
				return !settings.IncludePermanent
			}

			if ev.Status() == component.StatusRecoverableError && settings.IncludeRecoverable {
				return now.Before(ev.Timestamp().Add(settings.RecoveryDuration))
			}

			return ev.Status() != component.StatusFatalError
		}
	}
	return func(st *status.AggregateStatus) (int, *serializableStatus) {
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

		return code, sst
	}
}
