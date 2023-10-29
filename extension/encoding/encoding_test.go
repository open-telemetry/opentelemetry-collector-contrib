package encoding

import (
	"context"
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

type getEncodingFunc func(map[component.ID]component.Component, component.ID) error

func getEncoding[T any](extensions map[component.ID]component.Component, id component.ID) error {
	_, err := GetEncoding[T](extensions, id)
	return err
}

func TestMarshalUnmarshal(t *testing.T) {
	tests := []struct {
		name         string
		expectedErr  string
		id           component.ID
		getExtension func() (extension.Extension, error)
		cases        []getEncodingFunc
	}{
		{
			name: "otlp",
			id:   component.NewIDWithName(component.DataTypeTraces, "otlpencoding"),
			getExtension: func() (extension.Extension, error) {
				factory := otlpencodingextension.NewFactory()
				return factory.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), factory.CreateDefaultConfig())
			},
			// Supports logs, traces, metrics
			cases: []getEncodingFunc{
				getEncoding[ptrace.Marshaler],
				getEncoding[pmetric.Marshaler],
				getEncoding[plog.Marshaler],
				getEncoding[ptrace.Unmarshaler],
				getEncoding[pmetric.Unmarshaler],
				getEncoding[plog.Unmarshaler],
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
			cases: []getEncodingFunc{
				getEncoding[ptrace.Unmarshaler],
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
			cases: []getEncodingFunc{
				getEncoding[ptrace.Unmarshaler],
				getEncoding[ptrace.Marshaler],
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
			expectedErr: "extension \"traces/zipkinencoding\" doesn't implement",
			cases: []getEncodingFunc{
				getEncoding[plog.Unmarshaler],
				getEncoding[plog.Marshaler],
				getEncoding[pmetric.Marshaler],
				getEncoding[pmetric.Marshaler],
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
			cases: []getEncodingFunc{
				getEncoding[plog.Marshaler],
				getEncoding[plog.Unmarshaler],
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
			expectedErr: "extension \"logs/jsonlogencoding\" doesn't implement",
			cases: []getEncodingFunc{
				getEncoding[ptrace.Marshaler],
				getEncoding[ptrace.Unmarshaler],
				getEncoding[pmetric.Unmarshaler],
				getEncoding[pmetric.Unmarshaler],
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
			cases: []getEncodingFunc{
				getEncoding[plog.Marshaler],
				getEncoding[plog.Unmarshaler],
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
			expectedErr: "extension \"logs/textencoding\" doesn't implement",
			cases: []getEncodingFunc{
				getEncoding[ptrace.Marshaler],
				getEncoding[ptrace.Unmarshaler],
				getEncoding[pmetric.Unmarshaler],
				getEncoding[pmetric.Unmarshaler],
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
				err := c(host.ext, test.id)
				if test.expectedErr != "" {
					require.ErrorContains(t, err, test.expectedErr)
				} else {
					require.NoError(t, err)
				}
			}
		})
	}
}
