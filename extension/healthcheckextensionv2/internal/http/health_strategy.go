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

type healthStrategy interface {
	toResponse(*status.AggregateStatus) (int, *serializableStatus)
}

type defaultHealthStrategy struct {
	startTimestamp *time.Time
}

func (d *defaultHealthStrategy) toResponse(st *status.AggregateStatus) (int, *serializableStatus) {
	return responseCodes[st.Status()], toSerializableStatus(st, &serializationOptions{
		includeStartTime: true,
		startTimestamp:   d.startTimestamp,
	})
}

type componentHealthStrategy struct {
	settings       *common.ComponentHealthSettings
	startTimestamp *time.Time
}

func (c *componentHealthStrategy) toResponse(st *status.AggregateStatus) (int, *serializableStatus) {
	now := time.Now()
	sst := toSerializableStatus(
		st,
		&serializationOptions{
			includeStartTime: true,
			startTimestamp:   c.startTimestamp,
			healthyFunc:      c.healthyFunc(&now),
		},
	)

	if c.settings.IncludePermanent && st.Status() == component.StatusPermanentError {
		return http.StatusInternalServerError, sst
	}

	if c.settings.IncludeRecoverable && st.Status() == component.StatusRecoverableError {
		if now.After(st.Timestamp().Add(c.settings.RecoveryDuration)) {
			return http.StatusInternalServerError, sst
		}
	}

	return responseCodes[st.Status()], sst
}

func (c *componentHealthStrategy) healthyFunc(now *time.Time) func(status.Event) bool {
	return func(ev status.Event) bool {
		if ev.Status() == component.StatusPermanentError {
			return !c.settings.IncludePermanent
		}

		if ev.Status() == component.StatusRecoverableError && c.settings.IncludeRecoverable {
			return now.Before(ev.Timestamp().Add(c.settings.RecoveryDuration))
		}

		return ev.Status() != component.StatusFatalError
	}
}
