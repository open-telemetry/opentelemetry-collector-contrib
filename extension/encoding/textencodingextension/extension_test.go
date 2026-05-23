// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package textencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/textencodingextension"

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensiontest"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

func TestExtension_Start(t *testing.T) {
	tests := []struct {
		name         string
		getExtension func() (extension.Extension, error)
		expectedErr  string
	}{
		{
			name: "text",
			getExtension: func() (extension.Extension, error) {
				factory := NewFactory()
				return factory.Create(t.Context(), extensiontest.NewNopSettings(factory.Type()), factory.CreateDefaultConfig())
			},
		},
		{
			name: "text_gbk",
			getExtension: func() (extension.Extension, error) {
				factory := NewFactory()
				cfg := factory.CreateDefaultConfig()
				cfg.(*Config).Encoding = "gbk"
				return factory.Create(t.Context(), extensiontest.NewNopSettings(factory.Type()), cfg)
			},
		},
		{
			name: "text_blabla",
			getExtension: func() (extension.Extension, error) {
				factory := NewFactory()
				cfg := factory.CreateDefaultConfig()
				cfg.(*Config).Encoding = "blabla"
				return factory.Create(t.Context(), extensiontest.NewNopSettings(factory.Type()), cfg)
			},
			expectedErr: "unsupported encoding 'blabla'",
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
			err = ext.Start(t.Context(), componenttest.NewNopHost())
			if test.expectedErr != "" && err != nil {
				require.ErrorContains(t, err, test.expectedErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func Test_MarshalUnmarshal(t *testing.T) {
	factory := NewFactory()
	ext, err := factory.Create(t.Context(), extensiontest.NewNopSettings(factory.Type()), factory.CreateDefaultConfig())
	require.NoError(t, err)
	err = ext.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	e := ext.(*textExtension)
	logs := plog.NewLogs()
	lr := logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	lr.Body().SetStr("foo")
	b, err := e.MarshalLogs(logs)
	require.NoError(t, err)
	result, err := e.UnmarshalLogs(b)
	require.NoError(t, err)
	require.NoError(t, plogtest.CompareLogs(logs, result, plogtest.IgnoreTimestamp(), plogtest.IgnoreObservedTimestamp()))
}

func Test_StartSeparatorConfig(t *testing.T) {
	factory := NewFactory()
	ext, err := factory.Create(t.Context(), extensiontest.NewNopSettings(factory.Type()), factory.CreateDefaultConfig())
	require.NoError(t, err)
	err = ext.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	e := ext.(*textExtension)
	require.Equal(t, e.config.MarshalingSeparator, e.textEncoder.marshalingSeparator)
	require.Equal(t, e.config.UnmarshalingSeparator, e.textEncoder.unmarshalingSeparator.String())
}
