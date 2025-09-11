// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

func TestCollectorInstanceInfo(t *testing.T) {
	for _, tc := range []struct {
		name     string
		input    component.TelemetrySettings
		expected pcommon.Map
	}{
		{
			name:     "empty",
			input:    componenttest.NewNopTelemetrySettings(),
			expected: pcommon.NewMap(),
		},
		{
			name: "with_service_instance_id",
			input: func() component.TelemetrySettings {
				ts := componenttest.NewNopTelemetrySettings()
				ts.Resource.Attributes().PutStr(string(semconv.ServiceInstanceIDKey), "627cc493-f310-47de-96bd-71410b7dec09")
				return ts
			}(),
			expected: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr(
					"signaltometrics."+string(semconv.ServiceInstanceIDKey),
					"627cc493-f310-47de-96bd-71410b7dec09",
				)
				return m
			}(),
		},
		{
			name: "with_all_values",
			input: func() component.TelemetrySettings {
				ts := componenttest.NewNopTelemetrySettings()
				ts.Resource.Attributes().PutStr(string(semconv.ServiceInstanceIDKey), "627cc493-f310-47de-96bd-71410b7dec09")
				ts.Resource.Attributes().PutStr(string(semconv.ServiceNameKey), "signaltometrics")
				ts.Resource.Attributes().PutStr(string(semconv.ServiceNamespaceKey), "test")
				return ts
			}(),
			expected: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr(
					"signaltometrics."+string(semconv.ServiceInstanceIDKey),
					"627cc493-f310-47de-96bd-71410b7dec09",
				)
				m.PutStr(
					"signaltometrics."+string(semconv.ServiceNameKey),
					"signaltometrics",
				)
				m.PutStr(
					"signaltometrics."+string(semconv.ServiceNamespaceKey),
					"test",
				)
				return m
			}(),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ci := NewCollectorInstanceInfo(tc.input)
			require.NotNil(t, ci)

			actual := pcommon.NewMap()
			ci.Copy(actual)
			assert.Equal(t, ci.Size(), actual.Len())
			assertMapEquality(t, tc.expected, actual)
		})
	}
}

func assertMapEquality(t *testing.T, expected, actual pcommon.Map) bool {
	t.Helper()

	expectedRaw := expected.AsRaw()
	actualRaw := actual.AsRaw()
	return assert.True(
		t, reflect.DeepEqual(expectedRaw, actualRaw),
		"attributes don't match expected: %v, actual: %v",
		expectedRaw, actualRaw,
	)
}
