// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs

import (
	"encoding/json"
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

func TestAttrPutHTTPProtoIf(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		httpProtocol string
		wantRaw      map[string]any
	}{
		{
			name:         "valid HTTP 0.9 proto",
			httpProtocol: httpVersion09,
			wantRaw: map[string]any{
				"network.protocol.name":    "http",
				"network.protocol.version": "0.9",
			},
		},
		{
			name:         "valid HTTP 1.0 proto",
			httpProtocol: httpVersion10,
			wantRaw: map[string]any{
				"network.protocol.name":    "http",
				"network.protocol.version": "1.0",
			},
		},
		{
			name:         "valid HTTP 1.1 proto",
			httpProtocol: httpVersion11,
			wantRaw: map[string]any{
				"network.protocol.name":    "http",
				"network.protocol.version": "1.1",
			},
		},
		{
			name:         "valid HTTP 2.0 proto",
			httpProtocol: httpVersion20,
			wantRaw: map[string]any{
				"network.protocol.name":    "http",
				"network.protocol.version": "2.0",
			},
		},
		{
			name:         "valid HTTP 3.0 proto",
			httpProtocol: httpVersion30,
			wantRaw: map[string]any{
				"network.protocol.name":    "http",
				"network.protocol.version": "3.0",
			},
		},
		{
			name:         "HTTP proto without version",
			httpProtocol: "HTTP",
			wantRaw: map[string]any{
				attributeNetworkProtocolOriginal: "HTTP",
			},
		},
		{
			name:         "HTTP proto with extra data",
			httpProtocol: "HTTP/1.0 something",
			wantRaw: map[string]any{
				attributeNetworkProtocolOriginal: "HTTP/1.0 something",
			},
		},
		{
			name:         "unknown HTTP",
			httpProtocol: "ABC 3.4",
			wantRaw: map[string]any{
				attributeNetworkProtocolOriginal: "ABC 3.4",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pcommon.NewMap()
			attrPutHTTPProtoIf(got, tt.httpProtocol)
			want := pcommon.NewMap()
			err := want.FromRaw(tt.wantRaw)
			require.NoError(t, err)
			assert.Equal(t, want.AsRaw(), got.AsRaw())
		})
	}
}

func TestConvertInvalidSingleQuotedJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    []byte
		expected []byte
	}{
		{
			name:     "empty input",
			input:    []byte{},
			expected: []byte{},
		},
		{
			name:     "single quoted JSON as a string",
			input:    []byte(`{'key':'value'}`),
			expected: []byte(`{"key":"value"}`),
		},
		{
			name:     "invalid single quoted JSON",
			input:    []byte(`'key':'value'`),
			expected: []byte(`"key":"value"`),
		},
		{
			name:     "single quoted JSON with escaped single quote",
			input:    []byte(`{'key':'val\'ue'}`),
			expected: []byte(`{"key":"val'ue"}`),
		},
		{
			name:     "single quoted JSON with double escaped single quote",
			input:    []byte(`{'key':'val\\'ue'}`),
			expected: []byte(`{"key":"val\\'ue"}`),
		},
		{
			name:     "valid, double quoted JSON",
			input:    []byte(`{"key":"value"}`),
			expected: []byte(`{"key":"value"}`),
		},
		{
			name:     "mixed quotes",
			input:    []byte(`{"key":'value'}`),
			expected: []byte(`{"key":"value"}`),
		},
		{
			name:     "no quotes at all",
			input:    []byte(`{key:value}`),
			expected: []byte(`{key:value}`),
		},
		{
			name:     "single quoted with spaces",
			input:    []byte(`{  'key' : 'value'  }`),
			expected: []byte(`{  "key" : "value"  }`),
		},
		{
			name:     "single quoted with numbers",
			input:    []byte(`{'key': 3.14}`),
			expected: []byte(`{"key": 3.14}`),
		},
		{
			name:     "all-in-one case",
			input:    []byte(`{ 'a\'': "it's John's", "today's": '"forecast\"', 'pi': 3.14 }`),
			expected: []byte(`{ "a'": "it's John's", "today's": "\"forecast\"", "pi": 3.14 }`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertInvalidSingleQuotedJSON(tt.input)
			assert.Equal(t, string(tt.expected), string(got))
		})
	}
}

func Test_stringToNumber(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  json.Number
	}{
		{
			name:  "empty string",
			input: "",
			want:  json.Number(""),
		},
		{
			name:  "dash",
			input: "-",
			want:  json.Number(""),
		},
		{
			name:  "valid integer string",
			input: "123",
			want:  json.Number("123"),
		},
		{
			name:  "valid float string",
			input: "3.14",
			want:  json.Number("3.14"),
		},
		{
			name:  "valid scientific float string",
			input: "3.1415926535E+00",
			want:  json.Number("3.1415926535E+00"),
		},
		{
			name:  "whitespace string",
			input: " ",
			want:  json.Number(""),
		},
		{
			name:  "invalid string",
			input: "abc",
			want:  json.Number(""),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertStringToJSONNumber(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}
