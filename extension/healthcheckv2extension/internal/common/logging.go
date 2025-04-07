// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckv2extension/internal/common"

import (
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/status"
)

// HealthLogFields generates log fields for the health status of a pipeline instance.
// If verbosity is [status.Concise], only top level statuses are taken into
// account, i.e. pipelines, the extensions host. Otherwise it will include
// per pipeline component/extension details.
func HealthLogFields(instanceID string, st *status.AggregateStatus, verbosity status.Verbosity) []zap.Field {
	logFields := []zap.Field{
		zap.String("pipeline.instance.id", instanceID),
		zap.String("pipeline.status", st.Status().String()),
	}

	if stErr := st.Err(); stErr != nil {
		logFields = append(logFields, zap.String("pipeline.error", stErr.Error()))
	}

	if verbosity == status.Concise {
		return logFields
	}

	for component, componentSt := range st.ComponentStatusMap {
		logFields = append(logFields, zap.String(component, componentSt.Status().String()))
	}

	return logFields
}
