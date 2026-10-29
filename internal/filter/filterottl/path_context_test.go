// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filterottl

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func TestNewBoolExprForSpanWithPathContextNames(t *testing.T) {
	tests := []struct {
		name       string
		conditions []string
		wantErr    bool
	}{
		{
			name:       "prefixed_path",
			conditions: []string{`span.name == "foo"`},
		},
		{
			name:       "unprefixed_path",
			conditions: []string{`name == "foo"`},
		},
		{
			name:       "mixed_paths",
			conditions: []string{`span.name == "foo"`, `resource.attributes["host"] != nil`, `attributes["env"] == "prod"`},
		},
		{
			name:       "unknown_prefix",
			conditions: []string{`unknown.attributes["foo"] == "bar"`},
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := NewBoolExprForSpanWithPathContextNames(tt.conditions, StandardSpanFuncs(), ottl.PropagateError, componenttest.NewNopTelemetrySettings())
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, expr)
		})
	}
}

func TestNewBoolExprForSpanEventWithPathContextNames(t *testing.T) {
	expr, err := NewBoolExprForSpanEventWithPathContextNames(
		[]string{`spanevent.name == "foo"`, `attributes["env"] == "prod"`},
		StandardSpanEventFuncs(), ottl.PropagateError, componenttest.NewNopTelemetrySettings(),
	)
	require.NoError(t, err)
	require.NotNil(t, expr)
}

func TestNewBoolExprForMetricWithPathContextNames(t *testing.T) {
	expr, err := NewBoolExprForMetricWithPathContextNames(
		[]string{`metric.name == "foo"`, `name == "bar"`},
		StandardMetricFuncs(), ottl.PropagateError, componenttest.NewNopTelemetrySettings(),
	)
	require.NoError(t, err)
	require.NotNil(t, expr)
}

func TestNewBoolExprForDataPointWithPathContextNames(t *testing.T) {
	expr, err := NewBoolExprForDataPointWithPathContextNames(
		[]string{`datapoint.attributes["foo"] != nil`, `attributes["bar"] != nil`},
		StandardDataPointFuncs(), ottl.PropagateError, componenttest.NewNopTelemetrySettings(),
	)
	require.NoError(t, err)
	require.NotNil(t, expr)
}

func TestNewBoolExprForLogWithPathContextNames(t *testing.T) {
	expr, err := NewBoolExprForLogWithPathContextNames(
		[]string{`log.severity_number >= SEVERITY_NUMBER_ERROR`, `attributes["env"] == "prod"`},
		StandardLogFuncs(), ottl.PropagateError, componenttest.NewNopTelemetrySettings(),
	)
	require.NoError(t, err)
	require.NotNil(t, expr)
}

func TestNewBoolExprForProfileWithPathContextNames(t *testing.T) {
	expr, err := NewBoolExprForProfileWithPathContextNames(
		[]string{`profile.duration_unix_nano > 0`, `attributes["env"] == "prod"`},
		StandardProfileFuncs(), ottl.PropagateError, componenttest.NewNopTelemetrySettings(),
	)
	require.NoError(t, err)
	require.NotNil(t, expr)
}

func TestNewBoolExprForResourceWithPathContextNames(t *testing.T) {
	expr, err := NewBoolExprForResourceWithPathContextNames(
		[]string{`resource.dropped_attributes_count == 0`, `attributes["env"] == "prod"`},
		StandardResourceFuncs(), ottl.PropagateError, componenttest.NewNopTelemetrySettings(),
	)
	require.NoError(t, err)
	require.NotNil(t, expr)
}

func TestNewBoolExprForScopeWithPathContextNames(t *testing.T) {
	expr, err := NewBoolExprForScopeWithPathContextNames(
		[]string{`scope.name != ""`, `attributes["env"] == "prod"`},
		StandardScopeFuncs(), ottl.PropagateError, componenttest.NewNopTelemetrySettings(),
	)
	require.NoError(t, err)
	require.NotNil(t, expr)
}
