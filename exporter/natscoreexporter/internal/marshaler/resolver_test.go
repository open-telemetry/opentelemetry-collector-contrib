// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package marshaler

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/testdata"
)

func TestBuiltinMarshalerResolver(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                 string
		builtinMarshalerName BuiltinMarshalerName
		wantLogsMarshaler    plog.Marshaler
		wantMetricsMarshaler pmetric.Marshaler
		wantTracesMarshaler  ptrace.Marshaler
		wantError            error
	}{
		{
			name:                 "resolves JSON marshaler for OtlpJSONBuiltinMarshalerName",
			builtinMarshalerName: OtlpJSONBuiltinMarshalerName,
			wantLogsMarshaler:    &plog.JSONMarshaler{},
			wantMetricsMarshaler: &pmetric.JSONMarshaler{},
			wantTracesMarshaler:  &ptrace.JSONMarshaler{},
			wantError:            nil,
		},
		{
			name:                 "resolves Protobuf marshaler for OtlpProtoBuiltinMarshalerName",
			builtinMarshalerName: OtlpProtoBuiltinMarshalerName,
			wantLogsMarshaler:    &plog.ProtoMarshaler{},
			wantMetricsMarshaler: &pmetric.ProtoMarshaler{},
			wantTracesMarshaler:  &ptrace.ProtoMarshaler{},
			wantError:            nil,
		},
		{
			name:                 "returns error for unsupported built-in marshaler",
			builtinMarshalerName: "unsupported",
			wantError:            errors.New("unsupported built-in marshaler"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver, err := NewBuiltinMarshalerResolver(tt.builtinMarshalerName)
			if tt.wantError != nil {
				assert.ErrorContains(t, err, tt.wantError.Error())
			} else {
				assert.NoError(t, err)

				genericMarshaler, err := resolver.Resolve(componenttest.NewNopHost())
				assert.NoError(t, err)

				genericBuiltinMarshaler, ok := genericMarshaler.(*genericBuiltinMarshaler)
				assert.True(t, ok)

				logs := testdata.GenerateLogs(10)
				metrics := testdata.GenerateMetrics(10)
				traces := testdata.GenerateTraces(10)

				haveEncodedLogs, err := genericBuiltinMarshaler.MarshalLogs(logs)
				assert.NoError(t, err)
				haveEncodedMetrics, err := genericBuiltinMarshaler.MarshalMetrics(metrics)
				assert.NoError(t, err)
				haveEncodedTraces, err := genericBuiltinMarshaler.MarshalTraces(traces)
				assert.NoError(t, err)

				wantEncodedLogs, err := tt.wantLogsMarshaler.MarshalLogs(logs)
				assert.NoError(t, err)
				wantEncodedMetrics, err := tt.wantMetricsMarshaler.MarshalMetrics(metrics)
				assert.NoError(t, err)
				wantEncodedTraces, err := tt.wantTracesMarshaler.MarshalTraces(traces)
				assert.NoError(t, err)

				assert.Equal(t, wantEncodedLogs, haveEncodedLogs)
				assert.Equal(t, wantEncodedMetrics, haveEncodedMetrics)
				assert.Equal(t, wantEncodedTraces, haveEncodedTraces)
			}
		})
	}
}

type fakeHost struct {
	extensions map[component.ID]component.Component
}

func (h *fakeHost) GetExtensions() map[component.ID]component.Component {
	return h.extensions
}

var _ component.Host = (*fakeHost)(nil)

type fakeExtension struct {
	component.Component
}

var _ component.Component = (*fakeExtension)(nil)

func TestEncodingExtensionResolver(t *testing.T) {
	t.Parallel()

	extension := &fakeExtension{}
	host := &fakeHost{
		extensions: map[component.ID]component.Component{
			component.NewID(component.MustNewType("extension")): extension,
		},
	}

	t.Run("resolves extension if found", func(t *testing.T) {
		resolver, err := NewEncodingExtensionResolver("extension")
		assert.NoError(t, err)

		extension, err := resolver.Resolve(host)
		assert.NoError(t, err)
		assert.Equal(t, extension, extension)
	})

	t.Run("returns error if extension not found", func(t *testing.T) {
		resolver, err := NewEncodingExtensionResolver("missing")
		assert.NoError(t, err)

		_, err = resolver.Resolve(host)
		assert.ErrorContains(t, err, "encoding extension not found")
	})
}
