// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckv2extension/internal/common"

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/status"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.uber.org/zap"
)

func TestHealthLogFields(t *testing.T) {
	tests := []struct {
		name       string
		instanceID string
		st         *status.AggregateStatus
		verbosity  status.Verbosity
		want       []zap.Field
	}{
		{
			name:       "simple and concise",
			instanceID: "test-instance",
			st:         makeSimpleStatus(componentstatus.StatusOK),
			verbosity:  status.Concise,
			want: []zap.Field{
				zap.String("pipeline.instance.id", "test-instance"),
				zap.String("pipeline.status", componentstatus.StatusOK.String()),
			},
		},
		{
			name:       "simple and verbose",
			instanceID: "test-instance",
			st:         makeSimpleStatus(componentstatus.StatusOK),
			verbosity:  status.Verbose,
			want: []zap.Field{
				zap.String("pipeline.instance.id", "test-instance"),
				zap.String("pipeline.status", componentstatus.StatusOK.String()),
			},
		},
		{
			name:       "complex and concise",
			instanceID: "test-instance",
			st: makeComplexStatus(
				componentstatus.StatusOK,
				map[string]*status.AggregateStatus{
					"component1": makeSimpleStatus(componentstatus.StatusOK),
					"component2": makeSimpleStatus(componentstatus.StatusFatalError),
				},
			),
			verbosity: status.Concise,
			want: []zap.Field{
				zap.String("pipeline.instance.id", "test-instance"),
				zap.String("pipeline.status", componentstatus.StatusOK.String()),
			},
		},
		{
			name:       "complex and verbose",
			instanceID: "test-instance",
			st: makeComplexStatus(
				componentstatus.StatusOK,
				map[string]*status.AggregateStatus{
					"component1": makeSimpleStatus(componentstatus.StatusOK),
					"component2": makeSimpleStatus(componentstatus.StatusFatalError),
				},
			),
			verbosity: status.Verbose,
			want: []zap.Field{
				zap.String("pipeline.instance.id", "test-instance"),
				zap.String("pipeline.status", componentstatus.StatusOK.String()),
				zap.String("component1", componentstatus.StatusOK.String()),
				zap.String("component2", componentstatus.StatusFatalError.String()),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HealthLogFields(tt.instanceID, tt.st, tt.verbosity)
			assert.ElementsMatch(t, got, tt.want)
		})
	}
}

func makeSimpleStatus(st componentstatus.Status) *status.AggregateStatus {
	return &status.AggregateStatus{
		Event:              componentstatus.NewEvent(st),
		ComponentStatusMap: map[string]*status.AggregateStatus{},
	}
}

func makeComplexStatus(st componentstatus.Status, innerSt map[string]*status.AggregateStatus) *status.AggregateStatus {
	return &status.AggregateStatus{
		Event:              componentstatus.NewEvent(st),
		ComponentStatusMap: innerSt,
	}
}
