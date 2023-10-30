// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package encoding

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensiontest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/jaegerencodingextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/jsonlogencodingextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/otlpencodingextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/textencodingextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/zipkinencodingextension"
)

type mockHost struct {
	component.Host
	ext map[component.ID]component.Component
}

func (mh mockHost) GetExtensions() map[component.ID]component.Component {
	return mh.ext
}

func TestEncodingStart(t *testing.T) {
	tests := []struct {
		name         string
		getExtension func() (extension.Extension, error)
		expectedErr  string
	}{
		{
			name: "otlpJson",
			getExtension: func() (extension.Extension, error) {
				factory := otlpencodingextension.NewFactory()
				cfg := factory.CreateDefaultConfig()
				cfg.(*otlpencodingextension.Config).Protocol = "otlp_json"
				return factory.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), cfg)
			},
		},
		{
			name: "otlpProtobuf",
			getExtension: func() (extension.Extension, error) {
				factory := otlpencodingextension.NewFactory()
				cfg := factory.CreateDefaultConfig()
				cfg.(*otlpencodingextension.Config).Protocol = "otlp_proto"
				return factory.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), cfg)
			},
		},
		{
			name: "zipkinJSON",
			getExtension: func() (extension.Extension, error) {
				factory := zipkinencodingextension.NewFactory()
				cfg := factory.CreateDefaultConfig()
				cfg.(*zipkinencodingextension.Config).Protocol = "zipkin_json"
				return factory.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), cfg)
			},
		},
		{
			name: "zipkinProtobuf",
			getExtension: func() (extension.Extension, error) {
				factory := zipkinencodingextension.NewFactory()
				cfg := factory.CreateDefaultConfig()
				cfg.(*zipkinencodingextension.Config).Protocol = "zipkin_proto"
				return factory.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), cfg)
			},
		},
		{
			name: "zipkinProtobuf",
			getExtension: func() (extension.Extension, error) {
				factory := zipkinencodingextension.NewFactory()
				cfg := factory.CreateDefaultConfig()
				cfg.(*zipkinencodingextension.Config).Protocol = "zipkin_thrift"
				cfg.(*zipkinencodingextension.Config).Version = "v1"
				return factory.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), cfg)
			},
		},
		{
			name: "jaegerProtobuf",
			getExtension: func() (extension.Extension, error) {
				factory := jaegerencodingextension.NewFactory()
				return factory.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), factory.CreateDefaultConfig())
			},
		},
		{
			name: "jsonLog",
			getExtension: func() (extension.Extension, error) {
				factory := jsonlogencodingextension.NewFactory()
				return factory.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), factory.CreateDefaultConfig())
			},
		},
		{
			name: "text",
			getExtension: func() (extension.Extension, error) {
				factory := textencodingextension.NewFactory()
				return factory.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), factory.CreateDefaultConfig())
			},
		},
		{
			name: "text_gbk",
			getExtension: func() (extension.Extension, error) {
				factory := textencodingextension.NewFactory()
				cfg := factory.CreateDefaultConfig()
				cfg.(*textencodingextension.Config).Encoding = "gbk"
				return factory.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), cfg)
			},
		},
		{
			name: "otlpProtocol_invalid",
			getExtension: func() (extension.Extension, error) {
				factory := otlpencodingextension.NewFactory()
				cfg := factory.CreateDefaultConfig()
				cfg.(*otlpencodingextension.Config).Protocol = "otlp_xyz"
				return factory.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), cfg)
			},
			expectedErr: "unsupported protocol: \"otlp_xyz\"",
		},
		{
			name: "zipkinThriftVersion_invalid",
			getExtension: func() (extension.Extension, error) {
				factory := zipkinencodingextension.NewFactory()
				cfg := factory.CreateDefaultConfig()
				cfg.(*zipkinencodingextension.Config).Protocol = "zipkin_thrift"
				cfg.(*zipkinencodingextension.Config).Version = "v2"
				return factory.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), cfg)
			},
			expectedErr: "unsupported version: \"v2\"",
		},
		{
			name: "jaeger_invalid",
			getExtension: func() (extension.Extension, error) {
				factory := jaegerencodingextension.NewFactory()
				cfg := factory.CreateDefaultConfig()
				cfg.(*jaegerencodingextension.Config).Protocol = "xyz"
				return factory.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), cfg)
			},
			expectedErr: "unsupported protocol: \"xyz\"",
		},
		{
			name: "text_invalid",
			getExtension: func() (extension.Extension, error) {
				factory := textencodingextension.NewFactory()
				cfg := factory.CreateDefaultConfig()
				cfg.(*textencodingextension.Config).Encoding = "invalid"
				return factory.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), cfg)
			},
			expectedErr: "unsupported encoding 'invalid'",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ext, err := test.getExtension()
			if test.expectedErr != "" && err != nil {
				require.ErrorContains(t, err, test.expectedErr)
			} else {
				require.NoError(t, err)
			}
			err = ext.Start(context.Background(), componenttest.NewNopHost())
			if test.expectedErr != "" && err != nil {
				require.ErrorContains(t, err, test.expectedErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

type marshalUnmarshalFn func(*testing.T, map[component.ID]component.Component, component.ID) error

func marshalUnmarshal[T any](t *testing.T, extensions map[component.ID]component.Component, id component.ID) error {
	ext, ok := extensions[id]
	require.NotNil(t, ext)
	require.True(t, ok)
	if _, ok := ext.(T); !ok {
		return fmt.Errorf("%s doesn't implement %T", id, (*T)(nil))
	}
	return nil
}

func TestMarshalUnmarshal(t *testing.T) {
	tests := []struct {
		name         string
		expectedErr  string
		id           component.ID
		getExtension func() (extension.Extension, error)
		cases        []marshalUnmarshalFn
	}{
		{
			name: "otlp",
			id:   component.NewIDWithName(component.DataTypeTraces, "otlpencoding"),
			getExtension: func() (extension.Extension, error) {
				factory := otlpencodingextension.NewFactory()
				return factory.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), factory.CreateDefaultConfig())
			},
			// Supports logs, traces, metrics
			cases: []marshalUnmarshalFn{
				marshalUnmarshal[ptrace.Marshaler],
				marshalUnmarshal[pmetric.Marshaler],
				marshalUnmarshal[plog.Marshaler],
				marshalUnmarshal[ptrace.Unmarshaler],
				marshalUnmarshal[pmetric.Unmarshaler],
				marshalUnmarshal[plog.Unmarshaler],
			},
		},
		{
			name: "jaeger",
			id:   component.NewIDWithName(component.DataTypeTraces, "jaegerencoding"),
			getExtension: func() (extension.Extension, error) {
				factory := jaegerencodingextension.NewFactory()
				return factory.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), factory.CreateDefaultConfig())
			},
			// Supports only traces (only unmarshaling)
			cases: []marshalUnmarshalFn{
				marshalUnmarshal[ptrace.Unmarshaler],
			},
		},
		{
			name: "zipkin",
			id:   component.NewIDWithName(component.DataTypeTraces, "zipkinencoding"),
			getExtension: func() (extension.Extension, error) {
				factory := zipkinencodingextension.NewFactory()
				return factory.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), factory.CreateDefaultConfig())
			},
			// Supports only traces
			cases: []marshalUnmarshalFn{
				marshalUnmarshal[ptrace.Unmarshaler],
				marshalUnmarshal[ptrace.Marshaler],
			},
		},
		{
			name: "zipkin",
			id:   component.NewIDWithName(component.DataTypeTraces, "zipkinencoding"),
			getExtension: func() (extension.Extension, error) {
				factory := zipkinencodingextension.NewFactory()
				return factory.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), factory.CreateDefaultConfig())
			},
			// Supports only traces, should raise error for logs/metrics
			expectedErr: "traces/zipkinencoding doesn't implement",
			cases: []marshalUnmarshalFn{
				marshalUnmarshal[plog.Unmarshaler],
				marshalUnmarshal[plog.Marshaler],
				marshalUnmarshal[pmetric.Marshaler],
				marshalUnmarshal[pmetric.Marshaler],
			},
		},
		{
			name: "jsonlog",
			id:   component.NewIDWithName(component.DataTypeLogs, "jsonlogencoding"),
			getExtension: func() (extension.Extension, error) {
				factory := jsonlogencodingextension.NewFactory()
				return factory.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), factory.CreateDefaultConfig())
			},
			// Supports only logs
			cases: []marshalUnmarshalFn{
				marshalUnmarshal[plog.Marshaler],
				marshalUnmarshal[plog.Unmarshaler],
			},
		},
		{
			name: "jsonlogError",
			id:   component.NewIDWithName(component.DataTypeLogs, "jsonlogencoding"),
			getExtension: func() (extension.Extension, error) {
				factory := jsonlogencodingextension.NewFactory()
				return factory.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), factory.CreateDefaultConfig())
			},
			// Supports only logs, should raise error for trace/metrics
			expectedErr: "logs/jsonlogencoding doesn't implement",
			cases: []marshalUnmarshalFn{
				marshalUnmarshal[ptrace.Marshaler],
				marshalUnmarshal[ptrace.Unmarshaler],
				marshalUnmarshal[pmetric.Unmarshaler],
				marshalUnmarshal[pmetric.Unmarshaler],
			},
		},
		{
			name: "text",
			id:   component.NewIDWithName(component.DataTypeLogs, "textencoding"),
			getExtension: func() (extension.Extension, error) {
				factory := textencodingextension.NewFactory()
				return factory.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), factory.CreateDefaultConfig())
			},
			// Supports only logs
			cases: []marshalUnmarshalFn{
				marshalUnmarshal[plog.Marshaler],
				marshalUnmarshal[plog.Unmarshaler],
			},
		},
		{
			name: "textError",
			id:   component.NewIDWithName(component.DataTypeLogs, "textencoding"),
			getExtension: func() (extension.Extension, error) {
				factory := textencodingextension.NewFactory()
				return factory.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), factory.CreateDefaultConfig())
			},
			// Supports only logs, should raise error for trace/metrics
			expectedErr: "logs/textencoding doesn't implement",
			cases: []marshalUnmarshalFn{
				marshalUnmarshal[ptrace.Marshaler],
				marshalUnmarshal[ptrace.Unmarshaler],
				marshalUnmarshal[pmetric.Unmarshaler],
				marshalUnmarshal[pmetric.Unmarshaler],
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ext, err := test.getExtension()
			host := mockHost{
				ext: map[component.ID]component.Component{
					test.id: ext,
				},
			}
			if test.expectedErr != "" && err != nil {
				require.ErrorContains(t, err, test.expectedErr)
			} else {
				require.NoError(t, err)
			}
			for _, c := range test.cases {
				err := c(t, host.ext, test.id)
				if test.expectedErr != "" {
					require.ErrorContains(t, err, test.expectedErr)
				} else {
					require.NoError(t, err)
				}
			}
		})
	}
}
