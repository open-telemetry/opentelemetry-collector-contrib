// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package simpleprometheusreceiver

import (
	"reflect"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/simpleprometheusreceiver/internal/metadata"
)

// TestStartLogsUnsupportedHTTPSettings checks that http client settings the
// receiver does not apply to the scrape are named in a debug log when the
// receiver starts.
func TestStartLogsUnsupportedHTTPSettings(t *testing.T) {
	tests := []struct {
		name     string
		conf     map[string]any
		wantKeys []any
	}{
		{
			name: "default config",
		},
		{
			name: "forwarded settings only",
			conf: map[string]any{
				"timeout":   "5s",
				"headers":   map[string]any{"X-Api-Token": "token"},
				"proxy_url": "http://proxy.example.com:3128",
			},
		},
		{
			name: "defaults written out",
			conf: map[string]any{
				"compression":         "",
				"force_attempt_http2": true,
				"keepalive":           map[string]any{"idle_conn_timeout": "90s", "max_idle_conns": 100},
			},
		},
		{
			name: "compression empty",
			conf: map[string]any{"compression": ""},
		},
		{
			name: "compression none",
			conf: map[string]any{"compression": "none"},
		},
		{
			name:     "compression gzip",
			conf:     map[string]any{"compression": "gzip"},
			wantKeys: []any{"compression"},
		},
		{
			name:     "auth with an undefined authenticator",
			conf:     map[string]any{"auth": map[string]any{"authenticator": "does_not_exist"}},
			wantKeys: []any{"auth"},
		},
		{
			name:     "keep-alives disabled",
			conf:     map[string]any{"keepalive": map[string]any{"enabled": false}},
			wantKeys: []any{"keepalive"},
		},
		{
			name:     "keep-alive setting changed",
			conf:     map[string]any{"keepalive": map[string]any{"max_idle_conns_per_host": 5}},
			wantKeys: []any{"keepalive"},
		},
		{
			name: "every ignored setting",
			conf: map[string]any{
				"timeout":                 "5s",
				"dialer_middleware":       map[string]any{"id": "dialer"},
				"read_buffer_size":        1024,
				"write_buffer_size":       1024,
				"auth":                    map[string]any{"authenticator": "bearertokenauth"},
				"compression":             "gzip",
				"compression_params":      map[string]any{"level": 1},
				"max_conns_per_host":      10,
				"http2_read_idle_timeout": "10s",
				"http2_ping_timeout":      "10s",
				"cookies":                 map[string]any{"enabled": true},
				"force_attempt_http2":     false,
				"middlewares":             []any{map[string]any{"id": "middleware"}},
				"keepalive":               map[string]any{"idle_conn_timeout": "5s"},
			},
			wantKeys: []any{
				"dialer_middleware", "read_buffer_size", "write_buffer_size", "auth",
				"compression", "compression_params", "max_conns_per_host",
				"http2_read_idle_timeout", "http2_ping_timeout", "cookies",
				"force_attempt_http2", "middlewares", "keepalive",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			require.NoError(t, confmap.NewFromStringMap(tt.conf).Unmarshal(cfg))
			cfg.ClientConfig.Endpoint = "localhost:1234"

			core, logs := observer.New(zapcore.DebugLevel)
			set := receivertest.NewNopSettings(metadata.Type)
			set.Logger = zap.New(core)
			r, err := NewFactory().CreateMetrics(t.Context(), set, cfg, consumertest.NewNop())
			require.NoError(t, err)
			require.NoError(t, r.Start(t.Context(), componenttest.NewNopHost()))
			require.NoError(t, r.Shutdown(t.Context()))

			logged := logs.FilterMessageSnippet("http client settings").All()
			if tt.wantKeys == nil {
				assert.Empty(t, logged)
				return
			}
			require.Len(t, logged, 1)
			assert.Equal(t, zapcore.DebugLevel, logged[0].Level)
			assert.Equal(t, tt.wantKeys, logged[0].ContextMap()["settings"])
		})
	}
}

// TestHTTPSettingsAreClassified fails when confighttp.ClientConfig gains a
// setting that this receiver does not handle, so the setting is not silently
// dropped.
func TestHTTPSettingsAreClassified(t *testing.T) {
	handled := map[string]string{
		// Forwarded to the prometheus scrape config.
		"endpoint":  "forwarded",
		"timeout":   "forwarded",
		"headers":   "forwarded",
		"proxy_url": "forwarded",
		// Forwarded or warned per setting; see TestTLSSettingsAreClassified.
		"tls": "classified by TestTLSSettingsAreClassified",
		// Not applied to the scrape; named in the unsupportedHTTPSettings debug log.
		"dialer_middleware":       "logged",
		"read_buffer_size":        "logged",
		"write_buffer_size":       "logged",
		"auth":                    "logged",
		"compression":             "logged",
		"compression_params":      "logged",
		"max_conns_per_host":      "logged",
		"http2_read_idle_timeout": "logged",
		"http2_ping_timeout":      "logged",
		"cookies":                 "logged",
		"force_attempt_http2":     "logged",
		"middlewares":             "logged",
		"keepalive":               "logged",
	}

	keys := configKeys(reflect.TypeFor[confighttp.ClientConfig]())
	for _, key := range keys {
		if _, ok := handled[key]; !ok {
			t.Errorf("http client key %q is not handled: forward it in getPrometheusConfig or add it to "+
				"unsupportedHTTPSettings, update README.md, then add it here", key)
		}
	}
	for key := range handled {
		if !slices.Contains(keys, key) {
			t.Errorf("http client key %q is listed here but confighttp.ClientConfig no longer has it", key)
		}
	}
}
