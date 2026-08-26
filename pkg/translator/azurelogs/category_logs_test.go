// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azurelogs

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

func TestPutInt(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		field              string
		value              string
		expectedAttributes map[string]any
		expectsErr         string
	}{
		"valid": {
			field: "test",
			value: "4",
			expectedAttributes: map[string]any{
				"test": int64(4),
			},
		},
		"invalid": {
			field:      "test",
			value:      "invalid",
			expectsErr: "failed to get number",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			record := plog.NewLogRecord()
			err := putInt(test.field, test.value, record)

			if test.expectsErr != "" {
				require.ErrorContains(t, err, test.expectsErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, test.expectedAttributes, record.Attributes().AsRaw())
		})
	}
}

func TestPutStr(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		field              string
		value              string
		expectedAttributes map[string]any
	}{
		"empty": {
			field:              "test",
			value:              "",
			expectedAttributes: map[string]any{},
		},
		"n/a": {
			field:              "test",
			value:              "N/A",
			expectedAttributes: map[string]any{},
		},
		"meaningful": {
			field: "test",
			value: "test",
			expectedAttributes: map[string]any{
				"test": "test",
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			record := plog.NewLogRecord()
			putStr(test.field, test.value, record)
			require.Equal(t, test.expectedAttributes, record.Attributes().AsRaw())
		})
	}
}

func TestPutBool(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		field              string
		value              string
		expectedAttributes map[string]any
	}{
		"true": {
			field: "test",
			value: "true",
			expectedAttributes: map[string]any{
				"test": true,
			},
		},
		"True": {
			field: "test",
			value: "True",
			expectedAttributes: map[string]any{
				"test": true,
			},
		},
		"TRUE": {
			field: "test",
			value: "TRUE",
			expectedAttributes: map[string]any{
				"test": true,
			},
		},
		"false": {
			field: "test",
			value: "false",
			expectedAttributes: map[string]any{
				"test": false,
			},
		},
		"False": {
			field: "test",
			value: "False",
			expectedAttributes: map[string]any{
				"test": false,
			},
		},
		"FALSE": {
			field: "test",
			value: "FALSE",
			expectedAttributes: map[string]any{
				"test": false,
			},
		},
		"invalid": {
			field:              "test",
			value:              "invalid",
			expectedAttributes: map[string]any{},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			record := plog.NewLogRecord()
			putBool(test.field, test.value, record)
			require.Equal(t, test.expectedAttributes, record.Attributes().AsRaw())
		})
	}
}

func TestHandleTime(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		field              string
		value              string
		expectedAttributes map[string]any
		expectsErr         string
	}{
		"valid": {
			field: "test",
			value: "0.154",
			expectedAttributes: map[string]any{
				"test": int64(154),
			},
		},
		"valid_2": {
			field: "test",
			value: "0.1546",
			expectedAttributes: map[string]any{
				"test": int64(154),
			},
		},
		"invalid": {
			field:      "test",
			value:      "invalid",
			expectsErr: "failed to get number",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			record := plog.NewLogRecord()
			err := handleTime(test.field, test.value, record)

			if test.expectsErr != "" {
				require.ErrorContains(t, err, test.expectsErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, test.expectedAttributes, record.Attributes().AsRaw())
		})
	}
}

func TestAddSecurityProtocolProperties(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		securityProtocol string
		expectsErr       string
	}{
		"valid": {
			securityProtocol: "TLS 1.3",
		},
		"missing_version": {
			securityProtocol: "TLS",
			expectsErr:       "missing version",
		},
		"invalid_format": {
			securityProtocol: "TLS 1.3 invalid",
			expectsErr:       "invalid format",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			record := plog.NewLogRecord()
			err := addSecurityProtocolProperties(test.securityProtocol, record)
			if test.expectsErr != "" {
				require.ErrorContains(t, err, test.expectsErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestHandleDestination(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		backendHostname string
		endpoint        string
		result          pcommon.Map
	}{
		"without_backend_hostname": {
			endpoint: "opentelemetry-cdn-endpoint.azureedge.net",
			result: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("destination.address", "opentelemetry-cdn-endpoint.azureedge.net")
				return m
			}(),
		},
		"without_both": {
			result: pcommon.NewMap(),
		},
		"without_endpoint": {
			backendHostname: "example.com:443",
			result: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutInt("destination.port", 443)
				m.PutStr("destination.address", "example.com")
				return m
			}(),
		},
		"both_equal": {
			backendHostname: "opentelemetry-cdn-endpoint.azureedge.net",
			endpoint:        "opentelemetry-cdn-endpoint.azureedge.net",
			result: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("destination.address", "opentelemetry-cdn-endpoint.azureedge.net")
				return m
			}(),
		},
		"both_different": {
			backendHostname: "example.com:443",
			endpoint:        "opentelemetry-cdn-endpoint.azureedge.net:443",
			result: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutInt("destination.port", 443)
				m.PutStr("destination.address", "example.com")
				m.PutStr("network.peer.address", "opentelemetry-cdn-endpoint.azureedge.net")
				m.PutInt("network.peer.port", 443)
				return m
			}(),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			record := plog.NewLogRecord()
			err := handleDestination(test.backendHostname, test.endpoint, record)
			require.NoError(t, err)
			require.Equal(t, test.result.AsRaw(), record.Attributes().AsRaw())
		})
	}
}
