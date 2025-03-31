// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckv2extension/internal/common"

import (
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/status"
	"go.uber.org/zap"
)

func HealthLogFields(prefix, instanceID string, st *status.AggregateStatus) []zap.Field {
	logFields := []zap.Field{
		zap.String(fmt.Sprintf("%s.instance.id", prefix), instanceID),
		zap.String(fmt.Sprintf("%s.status", prefix), st.Status().String()),
	}
	if stErr := st.Err(); stErr != nil {
		logFields = append(logFields, zap.String(fmt.Sprintf("%s.error", prefix), stErr.Error()))
	}
	return logFields
}
