// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudpubsubpushreceiver

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
)

func TestLoadEncodingExtension(t *testing.T) {
	encodingExt := mockEncodingExtension{}
	mHost := &mockHost{
		extensions: map[component.ID]component.Component{
			component.MustNewID("test_fail"):    nil,
			component.MustNewID("test_succeed"): encodingExt,
		},
	}

	_, err := loadEncodingExtension[encoding.LogsUnmarshalerExtension](mHost, component.ID{}, "test")
	require.ErrorContains(t, err, `extension "" not found`)

	_, err = loadEncodingExtension[encoding.LogsUnmarshalerExtension](mHost, component.MustNewID("test_fail"), "test")
	require.ErrorContains(t, err, `extension "test_fail" is not a test unmarshaler`)

	res, err := loadEncodingExtension[encoding.LogsUnmarshalerExtension](mHost, component.MustNewID("test_succeed"), "test")
	require.NoError(t, err)
	require.Equal(t, encodingExt, res)
}

type mockHost struct {
	extensions map[component.ID]component.Component
}

func (m *mockHost) GetExtensions() map[component.ID]component.Component {
	return m.extensions
}

type mockEncodingExtension struct {
	extension.Extension
}

var _ encoding.LogsUnmarshalerExtension = (*mockEncodingExtension)(nil)

func (mockEncodingExtension) UnmarshalLogs(_ []byte) (plog.Logs, error) {
	return plog.Logs{}, nil
}
