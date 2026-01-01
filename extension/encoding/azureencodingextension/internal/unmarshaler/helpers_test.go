// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package unmarshaler

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestDetectWrapperFormat(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		want    RecordsBatchFormat
		wantErr bool
	}{
		{
			name:    "empty input",
			input:   []byte{},
			want:    FormatUnknown,
			wantErr: false,
		},
		{
			name:    "JSON array format",
			input:   []byte(`[{"foo":"bar"},{"baz":"qux"}]`),
			want:    FormatJSONArray,
			wantErr: false,
		},
		{
			name:    "JSON object with records field",
			input:   []byte(`{"records":[{"foo":"bar"}]}`),
			want:    FormatObjectRecords,
			wantErr: false,
		},
		{
			name:    "newline delimiter JSON format",
			input:   []byte("{\"foo\":\"bar\"}\n{\"baz\":\"qux\"}"),
			want:    FormatNDJSON,
			wantErr: false,
		},
		{
			name:    "extra whitespaces",
			input:   []byte("{\n  \"records\" :\r\n [ ] }"),
			want:    FormatObjectRecords,
			wantErr: false,
		},
		{
			name:    "Unsupported JSON format",
			input:   []byte(`{"data":[{"foo":"bar"}]}`),
			want:    FormatUnknown,
			wantErr: true,
		},
		{
			name:    "JSON object with non-array records field",
			input:   []byte(`{"records":"foo"}`),
			want:    FormatUnknown,
			wantErr: true,
		},
		{
			name:    "not a JSON",
			input:   []byte(`foo`),
			want:    FormatUnknown,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DetectWrapperFormat(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAsTimestamp(t *testing.T) {
	type args struct {
		s       string
		formats []string
	}
	tests := []struct {
		name    string
		args    args
		want    pcommon.Timestamp
		wantErr bool
	}{
		{
			name: "ISO8601 format",
			args: args{
				s: "2025-01-02T12:34:56.000000Z",
			},
			want:    pcommon.NewTimestampFromTime(time.Date(2025, time.January, 2, 12, 34, 56, 0, time.UTC)),
			wantErr: false,
		},
		{
			name: "ISO8601 format without fractional seconds",
			args: args{
				s: "2025-01-02T12:34:56Z",
			},
			want:    pcommon.NewTimestampFromTime(time.Date(2025, time.January, 2, 12, 34, 56, 0, time.UTC)),
			wantErr: false,
		},
		{
			name: "Custom format",
			args: args{
				s:       "02/03/2025 12:34:56",
				formats: []string{"02/01/2006 15:04:05"},
			},
			want:    pcommon.NewTimestampFromTime(time.Date(2025, time.March, 2, 12, 34, 56, 0, time.UTC)),
			wantErr: false,
		},
		{
			name: "Multiple formats",
			args: args{
				s:       "02/03/2025 12:34:56.123 PM -01:00",
				formats: []string{"01/02/2006 15:04:05", "1/2/2006 3:04:05.000 PM -07:00"},
			},
			want:    pcommon.NewTimestampFromTime(time.Date(2025, time.February, 3, 13, 34, 56, 123000000, time.UTC)),
			wantErr: false,
		},
		{
			name: "Default ISO8601 format",
			args: args{
				s:       "2025-04-05T12:34:56.000000Z",
				formats: []string{"01/02/2006 15:04:05"},
			},
			want:    pcommon.NewTimestampFromTime(time.Date(2025, time.April, 5, 12, 34, 56, 0, time.UTC)),
			wantErr: false,
		},
		{
			name: "Invalid format",
			args: args{
				s: "string",
			},
			want:    0,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := AsTimestamp(tt.args.s, tt.args.formats...)
			if tt.wantErr {
				require.Error(t, err)
			}
			assert.Equal(t, tt.want.String(), got.String())
		})
	}
}

func TestAttrPutStrIf(t *testing.T) {
	type args struct {
		key   string
		value string
	}
	tests := []struct {
		name    string
		attrs   pcommon.Map
		args    args
		wantRaw map[string]any
	}{
		{
			name:  "Empty string",
			attrs: pcommon.NewMap(),
			args: args{
				key:   "attr1",
				value: "",
			},
			wantRaw: map[string]any{},
		},
		{
			name:  "Dash string",
			attrs: pcommon.NewMap(),
			args: args{
				key:   "attr1",
				value: unknownField,
			},
			wantRaw: map[string]any{},
		},
		{
			name:  "Value string",
			attrs: pcommon.NewMap(),
			args: args{
				key:   "attr2",
				value: "value",
			},
			wantRaw: map[string]any{"attr2": "value"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			AttrPutStrIf(tt.attrs, tt.args.key, tt.args.value)
			want := pcommon.NewMap()
			err := want.FromRaw(tt.wantRaw)
			require.NoError(t, err)
			assert.Equal(t, want.AsRaw(), tt.attrs.AsRaw())
		})
	}
}

func TestAttrPutStrPtrIf(t *testing.T) {
	emptyStr := ""
	dashStr := unknownField
	valueStr := "value"

	type args struct {
		key   string
		value *string
	}
	tests := []struct {
		name    string
		args    args
		wantRaw map[string]any
	}{
		{
			name: "No value",
			args: args{
				key:   "attr1",
				value: nil,
			},
			wantRaw: map[string]any{},
		},
		{
			name: "Empty string",
			args: args{
				key:   "attr2",
				value: &emptyStr,
			},
			wantRaw: map[string]any{},
		},
		{
			name: "Dash string",
			args: args{
				key:   "attr2",
				value: &dashStr,
			},
			wantRaw: map[string]any{},
		},
		{
			name: "Value string",
			args: args{
				key:   "attr3",
				value: &valueStr,
			},
			wantRaw: map[string]any{"attr3": "value"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pcommon.NewMap()
			AttrPutStrPtrIf(got, tt.args.key, tt.args.value)
			want := pcommon.NewMap()
			err := want.FromRaw(tt.wantRaw)
			require.NoError(t, err)
			assert.Equal(t, want.AsRaw(), got.AsRaw())
		})
	}
}

func TestAttrPutIntNumberIf(t *testing.T) {
	t.Parallel()

	type testNumStruct struct {
		Value json.Number `json:"value"`
	}

	tests := []struct {
		name      string
		inputJSON string
		wantRaw   map[string]any
	}{
		{
			name:      "string value",
			inputJSON: `{"value": "3"}`,
			wantRaw: map[string]any{
				"value": int64(3),
			},
		},
		{
			name:      "int value",
			inputJSON: `{"value": 10}`,
			wantRaw: map[string]any{
				"value": int64(10),
			},
		},
		{
			name:      "float value",
			inputJSON: `{"value": 3.14}`,
			wantRaw:   map[string]any{},
		},
		{
			name:      "no value",
			inputJSON: `{"novalue": 15}`,
			wantRaw:   map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ts testNumStruct
			err := json.Unmarshal([]byte(tt.inputJSON), &ts)
			require.NoError(t, err)

			got := pcommon.NewMap()
			AttrPutIntNumberIf(got, "value", ts.Value)
			want := pcommon.NewMap()
			err = want.FromRaw(tt.wantRaw)
			require.NoError(t, err)
			assert.Equal(t, want.AsRaw(), got.AsRaw())
		})
	}
}

func TestAttrPutIntNumberPtrIf(t *testing.T) {
	t.Parallel()

	type testNumStruct struct {
		Value *json.Number `json:"value"`
	}

	tests := []struct {
		name      string
		inputJSON string
		wantRaw   map[string]any
	}{
		{
			name:      "string value",
			inputJSON: `{"value": "3"}`,
			wantRaw: map[string]any{
				"value": int64(3),
			},
		},
		{
			name:      "int value",
			inputJSON: `{"value": 10}`,
			wantRaw: map[string]any{
				"value": int64(10),
			},
		},
		{
			name:      "float value",
			inputJSON: `{"value": 3.14}`,
			wantRaw:   map[string]any{},
		},
		{
			name:      "nil value",
			inputJSON: `{"novalue": 15}`,
			wantRaw:   map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ts testNumStruct
			err := json.Unmarshal([]byte(tt.inputJSON), &ts)
			require.NoError(t, err)

			got := pcommon.NewMap()
			AttrPutIntNumberPtrIf(got, "value", ts.Value)
			want := pcommon.NewMap()
			err = want.FromRaw(tt.wantRaw)
			require.NoError(t, err)
			assert.Equal(t, want.AsRaw(), got.AsRaw())
		})
	}
}

func TestAttrPutFloatNumberIf(t *testing.T) {
	t.Parallel()

	type testNumStruct struct {
		Value json.Number `json:"value"`
	}

	tests := []struct {
		name      string
		inputJSON string
		wantRaw   map[string]any
	}{
		{
			name:      "string int value",
			inputJSON: `{"value": "3"}`,
			wantRaw: map[string]any{
				"value": float64(3),
			},
		},
		{
			name:      "int value",
			inputJSON: `{"value": 10}`,
			wantRaw: map[string]any{
				"value": float64(10),
			},
		},
		{
			name:      "string float value",
			inputJSON: `{"value": "3.14"}`,
			wantRaw: map[string]any{
				"value": float64(3.14),
			},
		},
		{
			name:      "float value",
			inputJSON: `{"value": 2.71}`,
			wantRaw: map[string]any{
				"value": float64(2.71),
			},
		},
		{
			name:      "nil value",
			inputJSON: `{"novalue": 15}`,
			wantRaw:   map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ts testNumStruct
			err := json.Unmarshal([]byte(tt.inputJSON), &ts)
			require.NoError(t, err)

			got := pcommon.NewMap()
			AttrPutFloatNumberIf(got, "value", ts.Value)
			want := pcommon.NewMap()
			err = want.FromRaw(tt.wantRaw)
			require.NoError(t, err)
			assert.Equal(t, want.AsRaw(), got.AsRaw())
		})
	}
}

func TestAttrPutFloatNumberPtrIf(t *testing.T) {
	t.Parallel()

	type testNumStruct struct {
		Value *json.Number `json:"value"`
	}

	tests := []struct {
		name      string
		inputJSON string
		wantRaw   map[string]any
	}{
		{
			name:      "string int value",
			inputJSON: `{"value": "3"}`,
			wantRaw: map[string]any{
				"value": float64(3),
			},
		},
		{
			name:      "int value",
			inputJSON: `{"value": 10}`,
			wantRaw: map[string]any{
				"value": float64(10),
			},
		},
		{
			name:      "string float value",
			inputJSON: `{"value": "3.14"}`,
			wantRaw: map[string]any{
				"value": float64(3.14),
			},
		},
		{
			name:      "float value",
			inputJSON: `{"value": 2.71}`,
			wantRaw: map[string]any{
				"value": float64(2.71),
			},
		},
		{
			name:      "nil value",
			inputJSON: `{"novalue": 15}`,
			wantRaw:   map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ts testNumStruct
			err := json.Unmarshal([]byte(tt.inputJSON), &ts)
			require.NoError(t, err)

			got := pcommon.NewMap()
			AttrPutFloatNumberPtrIf(got, "value", ts.Value)
			want := pcommon.NewMap()
			err = want.FromRaw(tt.wantRaw)
			require.NoError(t, err)
			assert.Equal(t, want.AsRaw(), got.AsRaw())
		})
	}
}

func TestAttrPutMapIf(t *testing.T) {
	t.Parallel()

	type args struct {
		attrKey   string
		attrValue map[string]any
	}
	tests := []struct {
		name    string
		args    args
		wantRaw map[string]any
	}{
		{
			name: "no key",
			args: args{
				attrKey:   "",
				attrValue: map[string]any{"foo": "bar"},
			},
			wantRaw: map[string]any{},
		},
		{
			name: "no map",
			args: args{
				attrKey:   "foo",
				attrValue: nil,
			},
			wantRaw: map[string]any{},
		},
		{
			name: "map value",
			args: args{
				attrKey:   "map",
				attrValue: map[string]any{"x": 1},
			},
			wantRaw: map[string]any{
				"map": map[string]any{"x": 1},
			},
		},
		{
			name: "unsupported map value",
			args: args{
				attrKey: "map",
				attrValue: map[string]any{
					"struct": struct{ name string }{
						name: "test",
					},
				},
			},
			wantRaw: map[string]any{
				"map_original": "map[struct:{test}]",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pcommon.NewMap()
			AttrPutMapIf(got, tt.args.attrKey, tt.args.attrValue)
			want := pcommon.NewMap()
			err := want.FromRaw(tt.wantRaw)
			require.NoError(t, err)
			assert.Equal(t, want.AsRaw(), got.AsRaw())
		})
	}
}

func TestAttrPutURLParsed(t *testing.T) {
	tests := []struct {
		name      string
		uri       string
		wantAttrs map[string]any
	}{
		{
			name:      "empty uri",
			uri:       "",
			wantAttrs: map[string]any{},
		},
		{
			name: "invalid url",
			uri:  "://bad-url",
			wantAttrs: map[string]any{
				"url.original": "://bad-url",
			},
		},
		{
			name: "url with invalid port",
			uri:  "http://example.com:abc/path",
			wantAttrs: map[string]any{
				"url.original": "http://example.com:abc/path",
			},
		},
		{
			name: "simple http url",
			uri:  "http://example.com/path?query=1#frag",
			wantAttrs: map[string]any{
				"url.full":     "http://example.com/path?query=1#frag",
				"url.scheme":   "http",
				"url.domain":   "example.com",
				"url.path":     "/path",
				"url.query":    "query=1",
				"url.fragment": "frag",
			},
		},
		{
			name: "url with port",
			uri:  "https://example.com:8080/path",
			wantAttrs: map[string]any{
				"url.full":   "https://example.com:8080/path",
				"url.scheme": "https",
				"url.domain": "example.com",
				"url.path":   "/path",
				"url.port":   int64(8080),
			},
		},
		{
			name: "url with credentials",
			uri:  "ftp://user:pass@example.com/resource",
			wantAttrs: map[string]any{
				"url.full":   "ftp://REDACTED:REDACTED@example.com/resource",
				"url.scheme": "ftp",
				"url.domain": "example.com",
				"url.path":   "/resource",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pcommon.NewMap()
			AttrPutURLParsed(got, tt.uri)
			want := pcommon.NewMap()
			err := want.FromRaw(tt.wantAttrs)
			require.NoError(t, err)
			assert.Equal(t, want.AsRaw(), got.AsRaw())
		})
	}
}

func TestAttrPutHostPortIf(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		addrKey string
		portKey string
		value   string
		wantRaw map[string]any
	}{
		{
			name:    "empty value",
			addrKey: "network.peer.address",
			portKey: "network.peer.port",
			value:   "",
			wantRaw: map[string]any{},
		},
		{
			name:    "host only",
			addrKey: "network.peer.address",
			portKey: "network.peer.port",
			value:   "example.com",
			wantRaw: map[string]any{
				"network.peer.address": "example.com",
			},
		},
		{
			name:    "host with valid port",
			addrKey: "destination.address",
			portKey: "destination.port",
			value:   "example.com:443",
			wantRaw: map[string]any{
				"destination.address": "example.com",
				"destination.port":    int64(443),
			},
		},
		{
			name:    "custom attribute names",
			addrKey: "address",
			portKey: "port",
			value:   "example.com:443",
			wantRaw: map[string]any{
				"address": "example.com",
				"port":    int64(443),
			},
		},
		{
			name:    "host with invalid port",
			addrKey: "client.address",
			portKey: "client.port",
			value:   "example.com:invalid",
			wantRaw: map[string]any{
				attributeClientAddressOriginal: "example.com:invalid",
			},
		},
		{
			name:    "IPv6 with port",
			addrKey: "server.address",
			portKey: "server.port",
			value:   "[2001:db8::1]:8080",
			wantRaw: map[string]any{
				"server.address": "2001:db8::1",
				"server.port":    int64(8080),
			},
		},
		{
			name:    "IPv6 without port",
			addrKey: "network.local.address",
			portKey: "network.local.port",
			value:   "[2001:db8::1]",
			wantRaw: map[string]any{
				"network.local.address": "[2001:db8::1]",
			},
		},
		{
			name:    "IPv6 without port and brackets",
			addrKey: "network.peer.address",
			portKey: "network.peer.port",
			value:   "2001:1c00:3280:6700:fbfa:bf04:1296:ebfc",
			wantRaw: map[string]any{
				"network.peer.address": "2001:1c00:3280:6700:fbfa:bf04:1296:ebfc",
			},
		},
		{
			name:    "host with empty port",
			addrKey: "network.peer.address",
			portKey: "network.peer.port",
			value:   "example.com:",
			wantRaw: map[string]any{
				attributeNetworkPeerAddressOriginal: "example.com:",
			},
		},
		{
			name:    "host with negative port",
			addrKey: "network.peer.address",
			portKey: "network.peer.port",
			value:   "example.com:-1",
			wantRaw: map[string]any{
				"network.peer.address": "example.com",
				"network.peer.port":    int64(-1),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pcommon.NewMap()
			AttrPutHostPortIf(got, tt.addrKey, tt.portKey, tt.value)
			want := pcommon.NewMap()
			err := want.FromRaw(tt.wantRaw)
			require.NoError(t, err)
			assert.Equal(t, want.AsRaw(), got.AsRaw())
		})
	}
}
