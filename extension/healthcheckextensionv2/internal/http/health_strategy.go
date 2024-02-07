// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package http // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2/internal/http"

import (
	"net/http"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2/internal/status"
	"go.opentelemetry.io/collector/component"
)

type healthStrategy interface {
	toResponse(*status.AggregateStatus, bool) (int, *serializableStatus)
}

type defaultHealthStrategy struct {
	startTimestamp *time.Time
}

func (d *defaultHealthStrategy) toResponse(st *status.AggregateStatus, verbose bool) (int, *serializableStatus) {
	return http.StatusOK, toSerializableStatus(st, &serializationOptions{
		verbose:          verbose,
		includeStartTime: true,
		startTimestamp:   d.startTimestamp,
	})
}

type componentHealthStrategy struct {
	settings       *common.ComponentHealthSettings
	startTimestamp *time.Time
}

func (c *componentHealthStrategy) toResponse(st *status.AggregateStatus, verbose bool) (int, *serializableStatus) {
	now := time.Now()
	sst := toSerializableStatus(
		st,
		&serializationOptions{
			verbose:          verbose,
			includeStartTime: true,
			startTimestamp:   c.startTimestamp,
			healthyFunc:      c.healthyFunc(&now),
		},
	)

	if c.settings.IncludePermanentErrors && st.Status() == component.StatusPermanentError {
		return http.StatusInternalServerError, sst
	}

	if c.settings.IncludeRecoverableErrors {
		recoverable, found := status.ActiveRecoverable(st)
		if found && now.After(recoverable.Timestamp().Add(c.settings.RecoveryDuration)) {
			return http.StatusInternalServerError, sst
		}
	}

	return http.StatusOK, sst
}

func (c *componentHealthStrategy) healthyFunc(now *time.Time) func(*component.StatusEvent) bool {
	return func(ev *component.StatusEvent) bool {
		if ev.Status() == component.StatusPermanentError {
			return !c.settings.IncludePermanentErrors
		}

		if ev.Status() == component.StatusRecoverableError && c.settings.IncludeRecoverableErrors {
			return now.Before(ev.Timestamp().Add(c.settings.RecoveryDuration))
		}

		return ev.Status() != component.StatusFatalError
	}
}
