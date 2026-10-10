// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package snmpreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver"

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver/internal/metadata"
)

func TestTrapConfigMarshalRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name, pollForm, pollVersion, trapVersion string
	}{
		{name: "legacy polling only", pollForm: "legacy", pollVersion: "v2c"},
		{name: "nested polling only", pollForm: "nested", pollVersion: "v2c"},
		{name: "traps only defaults", trapVersion: "defaults"},
		{name: "traps only v1 and v2c", trapVersion: "v2c"},
		{name: "legacy combined", pollForm: "legacy", pollVersion: "v2c", trapVersion: "v2c"},
		{name: "nested combined", pollForm: "nested", pollVersion: "v2c", trapVersion: "v2c"},
		{name: "traps only v3", trapVersion: "v3"},
		{name: "nested combined v3", pollForm: "nested", pollVersion: "v3", trapVersion: "v3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := map[string]any{}
			switch tc.pollForm {
			case "legacy":
				raw = pollTestOptions(tc.pollVersion)
			case "nested":
				raw["poll"] = pollTestOptions(tc.pollVersion)
			}
			if tc.trapVersion == "defaults" {
				raw["traps"] = map[string]any{}
			} else if tc.trapVersion != "" {
				raw["traps"] = trapTestOptions(tc.trapVersion)
			}
			cfg := pollTestLoadConfig(t, raw)
			require.NoError(t, confmap.Validate(cfg))
			encoded := confmap.New()
			require.NoError(t, encoded.Marshal(struct {
				SNMP *Config `mapstructure:"snmp"`
			}{SNMP: cfg}, xconfmap.WithUnredacted()))
			snmp, err := encoded.Sub("snmp")
			require.NoError(t, err)
			require.Equal(t, cfg.Poll != nil, snmp.IsSet("poll"))
			require.Equal(t, cfg.Traps != nil, snmp.IsSet("traps"))
			require.Equal(t, cfg.Traps != nil && cfg.Traps.V3 != nil, snmp.IsSet("traps::v3"))
			for _, key := range pollingConfigKeys {
				require.Equal(t, cfg.Poll == nil, snmp.IsSet(key), key)
			}
			reloaded := NewFactory().CreateDefaultConfig().(*Config)
			require.NoError(t, snmp.Unmarshal(reloaded))
			require.NoError(t, confmap.Validate(reloaded))
			require.Equal(t, cfg, reloaded)
			require.Equal(t, pollTestLoadConfig(t, raw), cfg, "marshaling must not mutate configuration")
		})
	}
}

func TestTrapConfigMarshalRedaction(t *testing.T) {
	for _, form := range []string{"traps only", "legacy combined", "nested combined"} {
		for _, mode := range []string{"redacted", "unredacted"} {
			t.Run(form+"/"+mode, func(t *testing.T) {
				raw := map[string]any{}
				switch form {
				case "legacy combined":
					raw = pollTestOptions("v3")
				case "nested combined":
					raw["poll"] = pollTestOptions("v3")
				}
				raw["traps"] = trapTestOptions("v3")
				cfg := pollTestLoadConfig(t, raw)
				var options []confmap.MarshalOption
				if mode == "unredacted" {
					options = append(options, xconfmap.WithUnredacted())
				}
				encoded := confmap.New()
				require.NoError(t, encoded.Marshal(struct {
					SNMP *Config `mapstructure:"snmp"`
				}{SNMP: cfg}, options...))
				communities := []any{"[REDACTED]", "[REDACTED]"}
				if mode == "unredacted" {
					communities = []any{"private-trap-community", "second-trap-community"}
				}
				require.Equal(t, communities, encoded.Get("snmp::traps::communities"))
				credentials := map[string]string{
					"snmp::traps::v3::auth_password":    "trap-auth-password",
					"snmp::traps::v3::privacy_password": "trap-privacy-password",
				}
				if form != "traps only" {
					prefix := "snmp::"
					if form == "nested combined" {
						prefix += "poll::"
					}
					credentials[prefix+"auth_password"] = "auth-password"
					credentials[prefix+"privacy_password"] = "privacy-password"
				}
				for key, original := range credentials {
					want := "[REDACTED]"
					if mode == "unredacted" {
						want = original
					}
					require.Equal(t, want, encoded.Get(key), key)
				}
				snmp, err := encoded.Sub("snmp")
				require.NoError(t, err)
				reloaded := NewFactory().CreateDefaultConfig().(*Config)
				require.NoError(t, snmp.Unmarshal(reloaded))
				require.NoError(t, confmap.Validate(reloaded))
			})
		}
	}
}

func trapTestOptions(version string) map[string]any {
	options := map[string]any{
		"listen_address":    "127.0.0.1:0",
		"versions":          []string{"v1", "v2c"},
		"communities":       []string{"private-trap-community", "second-trap-community"},
		"include_community": true,
		"queue_size":        32,
		"attributes":        map[string]any{"site": "lab"},
	}
	if version == "v3" {
		options["versions"] = []string{"v3"}
		options["v3"] = map[string]any{
			"user": "trap-user", "security_level": "auth_priv",
			"auth_type": "SHA256", "auth_password": "trap-auth-password",
			"privacy_type": "AES", "privacy_password": "trap-privacy-password",
		}
	}
	return options
}

func TestTrapConfigDefaultsAndSignalSelection(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	require.NoError(t, confmap.NewFromStringMap(map[string]any{}).Unmarshal(cfg))
	assert.Nil(t, cfg.Traps)
	_, err := factory.CreateLogs(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.ErrorContains(t, err, "traps must be configured")

	cfg = factory.CreateDefaultConfig().(*Config)
	require.NoError(t, confmap.NewFromStringMap(map[string]any{"traps": map[string]any{}}).Unmarshal(cfg))
	assert.Equal(t, defaultTrapsConfig(), cfg.Traps)
	require.NoError(t, confmap.Validate(cfg))
	logs, err := factory.CreateLogs(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NoError(t, logs.Shutdown(t.Context()))
	_, err = factory.CreateMetrics(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.ErrorIs(t, err, errMetricRequired)

	// The same configuration can still create the polling signal independently.
	cfg.Metrics = map[string]*MetricConfig{"device.uptime": {
		Unit: "1", Gauge: &GaugeMetric{ValueType: "int"}, ScalarOIDs: []ScalarOID{{OID: trapUptimeOID}},
	}}
	require.NoError(t, confmap.Validate(cfg))
	metrics, err := factory.CreateMetrics(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NoError(t, metrics.Shutdown(t.Context()))
}

func TestTrapV3ConfigDefaults(t *testing.T) {
	cfg := NewFactory().CreateDefaultConfig().(*Config)
	require.NoError(t, confmap.NewFromStringMap(map[string]any{"traps": map[string]any{
		"versions": []string{"v3"},
		"v3":       map[string]any{"user": "otel", "auth_password": "authpass123", "privacy_password": "privpass123"},
	}}).Unmarshal(cfg))
	require.NoError(t, confmap.Validate(cfg))
	assert.Equal(t, "auth_priv", cfg.Traps.V3.SecurityLevel)
	assert.Equal(t, "SHA256", cfg.Traps.V3.AuthType)
	assert.Equal(t, "AES", cfg.Traps.V3.PrivacyType)
}

func TestTrapConfigValidation(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change func(*TrapsConfig)
		want   string
	}{
		{name: "missing host", change: func(c *TrapsConfig) { c.ListenAddress = ":1620" }, want: "specify a host"},
		{name: "invalid port", change: func(c *TrapsConfig) { c.ListenAddress = "127.0.0.1:65536" }, want: "between 0 and 65535"},
		{name: "empty versions", change: func(c *TrapsConfig) { c.Versions = nil }, want: "at least one"},
		{name: "invalid version", change: func(c *TrapsConfig) { c.Versions = []string{"V2C"} }, want: "invalid traps version"},
		{name: "duplicate version", change: func(c *TrapsConfig) { c.Versions = []string{"v2c", "v2c"} }, want: "duplicate"},
		{name: "empty queue", change: func(c *TrapsConfig) { c.QueueSize = 0 }, want: "positive"},
		{name: "missing v3", change: func(c *TrapsConfig) { c.Versions = []string{"v3"} }, want: "must be configured"},
		{name: "unused v3", change: func(c *TrapsConfig) { c.V3 = &TrapV3Config{} }, want: "requires v3"},
		{name: "reserved attribute", change: func(c *TrapsConfig) { c.Attributes = map[string]string{"snmp.version": "wrong"} }, want: "reserved"},
		{name: "reserved peer attribute", change: func(c *TrapsConfig) { c.Attributes = map[string]string{"network.peer.address": "wrong"} }, want: "reserved"},
		{name: "empty attribute", change: func(c *TrapsConfig) { c.Attributes = map[string]string{"": "wrong"} }, want: "empty or reserved"},
		{name: "empty community", change: func(c *TrapsConfig) { c.Communities = []configopaque.String{""} }, want: "empty community"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := defaultTrapsConfig()
			tt.change(cfg)
			require.ErrorContains(t, cfg.validate(), tt.want)
		})
	}
	// Port zero is supported for ephemeral listeners, including lifecycle tests.
	cfg := defaultTrapsConfig()
	cfg.ListenAddress = "127.0.0.1:0"
	require.NoError(t, cfg.validate())
}

func TestTrapV3CredentialBoundaries(t *testing.T) {
	valid := func() *TrapsConfig {
		cfg := defaultTrapsConfig()
		cfg.Versions = []string{"v3"}
		cfg.V3 = &TrapV3Config{
			User: strings.Repeat("u", 32), SecurityLevel: "auth_priv",
			AuthType: "SHA256", AuthPassword: "12345678",
			PrivacyType: "AES", PrivacyPassword: "abcdefgh",
		}
		return cfg
	}
	require.NoError(t, valid().validate())
	for _, tc := range []struct {
		name   string
		change func(*TrapV3Config)
		want   string
	}{
		{name: "long user", change: func(c *TrapV3Config) { c.User += "u" }, want: "at most 32 bytes"},
		{name: "short auth password", change: func(c *TrapV3Config) { c.AuthPassword = "1234567" }, want: "auth_password must contain at least 8 bytes"},
		{name: "short privacy password", change: func(c *TrapV3Config) { c.PrivacyPassword = "abcdefg" }, want: "privacy_password must contain at least 8 bytes"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := valid()
			tc.change(cfg.V3)
			require.ErrorContains(t, cfg.validate(), tc.want)
		})
	}
	// An unauthenticated listener does not require either password.
	cfg := valid()
	cfg.V3.SecurityLevel = "no_auth_no_priv"
	cfg.V3.AuthPassword, cfg.V3.PrivacyPassword = "", ""
	require.NoError(t, cfg.validate())
}
