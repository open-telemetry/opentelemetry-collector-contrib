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
				assert.IsType(t, tt.wantLogsMarshaler, genericBuiltinMarshaler.logsMarshaler)
				assert.IsType(t, tt.wantMetricsMarshaler, genericBuiltinMarshaler.metricsMarshaler)
				assert.IsType(t, tt.wantTracesMarshaler, genericBuiltinMarshaler.tracesMarshaler)
			}
		})
	}
}

type mockHost struct {
	extensions map[component.ID]component.Component
}

func (h *mockHost) GetExtensions() map[component.ID]component.Component {
	return h.extensions
}

var _ component.Host = (*mockHost)(nil)

type mockExtension struct {
	component.Component
}

var _ component.Component = (*mockExtension)(nil)

func TestEncodingExtensionResolver(t *testing.T) {
	t.Parallel()

	extension := &mockExtension{}
	host := &mockHost{
		extensions: map[component.ID]component.Component{
			component.NewID(component.MustNewType("extension")): extension,
		},
	}

	tests := []struct {
		name                  string
		encodingExtensionName string
		wantEncodingExtension component.Component
		wantNewResolverError  error
		wantResolveError      error
	}{
		{
			name:                  "resolves extension",
			encodingExtensionName: "extension",
			wantEncodingExtension: extension,
			wantNewResolverError:  nil,
			wantResolveError:      nil,
		},
		{
			name:                  "returns error for invalid encoding extension name",
			encodingExtensionName: "",
			wantNewResolverError:  errors.New("failed to unmarshal encoding extension name"),
			wantResolveError:      nil,
		},
		{
			name:                  "returns error for missing encoding extension",
			encodingExtensionName: "missing",
			wantNewResolverError:  nil,
			wantResolveError:      errors.New("encoding extension not found"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver, err := NewEncodingExtensionResolver(tt.encodingExtensionName)
			if tt.wantNewResolverError != nil {
				assert.ErrorContains(t, err, tt.wantNewResolverError.Error())
			} else {
				assert.NoError(t, err)

				extension, err := resolver.Resolve(host)
				if tt.wantResolveError != nil {
					assert.ErrorContains(t, err, tt.wantResolveError.Error())
				} else {
					assert.NoError(t, err)
					assert.Equal(t, tt.wantEncodingExtension, extension)
				}
			}
		})
	}
}
