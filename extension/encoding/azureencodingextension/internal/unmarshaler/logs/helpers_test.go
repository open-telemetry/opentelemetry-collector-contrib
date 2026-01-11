// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
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
				"tls.protocol.name":    "SSL",
				"tls.protocol.version": "3",
			},
		},
		{
			name:             "valid TLS 1.0 proto",
			securityProtocol: tlsVersionTLS10,
			wantRaw: map[string]any{
				"tls.protocol.name":    "TLS",
				"tls.protocol.version": "1.0",
			},
		},
		{
			name:             "valid TLS 1.1 proto",
			securityProtocol: tlsVersionTLS11,
			wantRaw: map[string]any{
				"tls.protocol.name":    "TLS",
				"tls.protocol.version": "1.1",
			},
		},
		{
			name:             "valid TLS 1.2 proto",
			securityProtocol: tlsVersionTLS12,
			wantRaw: map[string]any{
				"tls.protocol.name":    "TLS",
				"tls.protocol.version": "1.2",
			},
		},
		{
			name:             "valid TLS 1.3 proto",
			securityProtocol: tlsVersionTLS13,
			wantRaw: map[string]any{
				"tls.protocol.name":    "TLS",
				"tls.protocol.version": "1.3",
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
