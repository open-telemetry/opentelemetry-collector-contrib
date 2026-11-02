// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package natsreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestNewUnmarshaler(t *testing.T) {
	proto, json := &plog.ProtoUnmarshaler{}, &plog.JSONUnmarshaler{}
	ext := &logsEncoding{}
	host := &extensionHost{exts: map[component.ID]component.Component{
		component.MustNewID("fake_encoding"): ext,
	}}

	tests := []struct {
		name    string
		cfg     SignalConfig
		want    plog.Unmarshaler
		wantErr string
	}{
		{name: "default", cfg: SignalConfig{}, want: proto},
		{name: "otlp_proto", cfg: SignalConfig{Encoding: encodingOTLPProto}, want: proto},
		{name: "otlp_json", cfg: SignalConfig{Encoding: encodingOTLPJSON}, want: json},
		{name: "encoding extension", cfg: SignalConfig{EncodingExtension: "fake_encoding"}, want: ext},
		// Config validation rejects the next two; cover them in case it is bypassed.
		{
			name:    "unsupported encoding",
			cfg:     SignalConfig{Encoding: "avro"},
			wantErr: `unsupported encoding: "avro"`,
		},
		{
			name:    "malformed extension id",
			cfg:     SignalConfig{EncodingExtension: "foo/"},
			wantErr: "failed to parse encoding extension name",
		},
		{
			name:    "extension not found",
			cfg:     SignalConfig{EncodingExtension: "missing"},
			wantErr: "encoding extension not found: missing",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := newUnmarshaler[plog.Unmarshaler](&tt.cfg, host, "logs", proto, json)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				assert.Nil(t, u)
				return
			}
			require.NoError(t, err)
			assert.Same(t, tt.want, u)
		})
	}
}

// TestNewUnmarshaler_WrongSignal verifies that an extension that cannot
// unmarshal the requested signal is rejected with the signal named.
func TestNewUnmarshaler_WrongSignal(t *testing.T) {
	host := &extensionHost{exts: map[component.ID]component.Component{
		component.MustNewID("fake_encoding"): &logsEncoding{},
	}}
	cfg := SignalConfig{EncodingExtension: "fake_encoding"}

	_, err := newUnmarshaler[pmetric.Unmarshaler](&cfg, host, "metrics", &pmetric.ProtoUnmarshaler{}, &pmetric.JSONUnmarshaler{})
	require.EqualError(t, err, "encoding extension fake_encoding does not support unmarshaling metrics")
}

func TestEncodingName(t *testing.T) {
	assert.Equal(t, encodingOTLPProto, encodingName(&SignalConfig{}))
	assert.Equal(t, encodingOTLPJSON, encodingName(&SignalConfig{Encoding: encodingOTLPJSON}))
	assert.Equal(t, "fake_encoding/logs", encodingName(&SignalConfig{EncodingExtension: "fake_encoding/logs"}))
}
