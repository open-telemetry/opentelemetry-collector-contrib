// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"
)

func TestAsSeverity(t *testing.T) {
	t.Parallel()

	tests := map[string]plog.SeverityNumber{
		"Informational": plog.SeverityNumberInfo,
		"Warning":       plog.SeverityNumberWarn,
		"Error":         plog.SeverityNumberError,
		"Critical":      plog.SeverityNumberFatal,
		"unknown":       plog.SeverityNumberUnspecified,
		"9":             plog.SeverityNumberUnspecified,
	}

	for input, expected := range tests {
		t.Run(input, func(t *testing.T) {
			assert.Equal(t, expected, asSeverity(input))
		})
	}
}

func TestAttrPutTLSProtoIf(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		securityProtocol string
		wantRaw          map[string]any
	}{
		{
			name:             "valid SSLv3 proto",
			securityProtocol: tlsVersionSSLv3,
			wantRaw: map[string]any{
				string(conventions.TLSProtocolNameKey):    "SSL",
				string(conventions.TLSProtocolVersionKey): "3",
			},
		},
		{
			name:             "valid TLS 1.0 proto",
			securityProtocol: tlsVersionTLS10,
			wantRaw: map[string]any{
				string(conventions.TLSProtocolNameKey):    "TLS",
				string(conventions.TLSProtocolVersionKey): "1.0",
			},
		},
		{
			name:             "valid TLS 1.1 proto",
			securityProtocol: tlsVersionTLS11,
			wantRaw: map[string]any{
				string(conventions.TLSProtocolNameKey):    "TLS",
				string(conventions.TLSProtocolVersionKey): "1.1",
			},
		},
		{
			name:             "valid TLS 1.2 proto",
			securityProtocol: tlsVersionTLS12,
			wantRaw: map[string]any{
				string(conventions.TLSProtocolNameKey):    "TLS",
				string(conventions.TLSProtocolVersionKey): "1.2",
			},
		},
		{
			name:             "valid TLS 1.3 proto",
			securityProtocol: tlsVersionTLS13,
			wantRaw: map[string]any{
				string(conventions.TLSProtocolNameKey):    "TLS",
				string(conventions.TLSProtocolVersionKey): "1.3",
			},
		},
		{
			name:             "TLS proto without version",
			securityProtocol: "TLS",
			wantRaw: map[string]any{
				attributeTLSProtocolOriginal: "TLS",
			},
		},
		{
			name:             "TLS proto with extra data",
			securityProtocol: "TLS 1.1 something",
			wantRaw: map[string]any{
				attributeTLSProtocolOriginal: "TLS 1.1 something",
			},
		},
		{
			name:             "unknown TLS",
			securityProtocol: "ABC 3.4",
			wantRaw: map[string]any{
				attributeTLSProtocolOriginal: "ABC 3.4",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pcommon.NewMap()
			attrPutTLSProtoIf(got, tt.securityProtocol)
			want := pcommon.NewMap()
			err := want.FromRaw(tt.wantRaw)
			require.NoError(t, err)
			assert.Equal(t, want.AsRaw(), got.AsRaw())
		})
	}
}

func TestAttrPutDestination(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		backendHostname string
		endpoint        string
		wantRaw         map[string]any
	}{
		{
			name:     "without backendHostname",
			endpoint: "opentelemetry-cdn-endpoint.azureedge.net",
			wantRaw: map[string]any{
				string(conventions.DestinationAddressKey): "opentelemetry-cdn-endpoint.azureedge.net",
			},
		},
		{
			name:    "without both",
			wantRaw: map[string]any{},
		},
		{
			name:            "without endpoint",
			backendHostname: "example.com:443",
			wantRaw: map[string]any{
				string(conventions.DestinationAddressKey): "example.com",
				string(conventions.DestinationPortKey):    443,
			},
		},
		{
			name:            "both equal",
			backendHostname: "opentelemetry-cdn-endpoint.azureedge.net",
			endpoint:        "opentelemetry-cdn-endpoint.azureedge.net",
			wantRaw: map[string]any{
				string(conventions.DestinationAddressKey): "opentelemetry-cdn-endpoint.azureedge.net",
			},
		},
		{
			name:            "both different",
			backendHostname: "example.com:443",
			endpoint:        "opentelemetry-cdn-endpoint.azureedge.net:443",
			wantRaw: map[string]any{
				string(conventions.DestinationAddressKey): "example.com",
				string(conventions.DestinationPortKey):    443,
				string(conventions.NetworkPeerAddressKey): "opentelemetry-cdn-endpoint.azureedge.net",
				string(conventions.NetworkPeerPortKey):    443,
			},
		},
		{
			name:            "invalid backendHostname",
			backendHostname: "some-invalid:hostname",
			wantRaw: map[string]any{
				"destination.address.original": "some-invalid:hostname",
			},
		},
		{
			name:     "invalid endpoint",
			endpoint: "some-invalid:endpoint",
			wantRaw: map[string]any{
				"destination.address.original": "some-invalid:endpoint",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pcommon.NewMap()
			attrPutDestination(got, tt.backendHostname, tt.endpoint)
			want := pcommon.NewMap()
			err := want.FromRaw(tt.wantRaw)
			require.NoError(t, err)
			assert.Equal(t, want.AsRaw(), got.AsRaw())
		})
	}
}
