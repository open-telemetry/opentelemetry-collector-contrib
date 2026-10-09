// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package snmpreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver"

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver/internal/metadata"
)

const pollTestUptimeOID = "1.3.6.1.2.1.1.3.0"

func TestPollConfigMarshalRoundTrip(t *testing.T) {
	for _, form := range []string{"legacy", "nested"} {
		for _, version := range []string{"v2c", "v3"} {
			t.Run(form+"/"+version, func(t *testing.T) {
				raw := pollTestOptions(version)
				if form == "nested" {
					raw = map[string]any{"poll": raw}
				}
				cfg := pollTestLoadConfig(t, raw)
				encoded := confmap.New()
				require.NoError(t, encoded.Marshal(struct {
					SNMP *Config `mapstructure:"snmp"`
				}{SNMP: cfg}, xconfmap.WithUnredacted()))
				snmp, err := encoded.Sub("snmp")
				require.NoError(t, err)
				require.Equal(t, form == "nested", snmp.IsSet("poll"))
				for _, key := range pollingConfigKeys {
					require.Equal(t, form == "legacy", snmp.IsSet(key), key)
				}
				reloaded := NewFactory().CreateDefaultConfig().(*Config)
				require.NoError(t, snmp.Unmarshal(reloaded))
				require.NoError(t, confmap.Validate(reloaded))
				require.Equal(t, cfg, reloaded)
				require.Equal(t, pollTestLoadConfig(t, raw), cfg, "marshaling must not mutate configuration")
			})
		}
	}
}

func TestPollConfigMarshalRedaction(t *testing.T) {
	for _, form := range []string{"legacy", "nested"} {
		for _, mode := range []string{"redacted", "unredacted"} {
			t.Run(form+"/"+mode, func(t *testing.T) {
				raw := pollTestOptions("v3")
				if form == "nested" {
					raw = map[string]any{"poll": raw}
				}
				cfg := pollTestLoadConfig(t, raw)
				var options []confmap.MarshalOption
				if mode == "unredacted" {
					options = append(options, xconfmap.WithUnredacted())
				}
				encoded := confmap.New()
				require.NoError(t, encoded.Marshal(struct {
					SNMP *Config `mapstructure:"snmp"`
				}{SNMP: cfg}, options...))
				prefix := "snmp::"
				if form == "nested" {
					prefix += "poll::"
				}
				for _, credential := range []struct {
					key, original string
				}{
					{key: "auth_password", original: "auth-password"},
					{key: "privacy_password", original: "privacy-password"},
				} {
					want := "[REDACTED]"
					if mode == "unredacted" {
						want = credential.original
					}
					require.Equal(t, want, encoded.Get(prefix+credential.key))
				}
			})
		}
	}
}

func TestPollConfigLegacyNestedEquivalence(t *testing.T) {
	for _, version := range []string{"v2c", "v3"} {
		t.Run(version, func(t *testing.T) {
			legacy := pollTestLoadConfig(t, pollTestOptions(version))
			nested := pollTestLoadConfig(t, map[string]any{"poll": pollTestOptions(version)})
			require.Nil(t, legacy.Poll)
			require.NotNil(t, nested.Poll)
			require.NoError(t, confmap.Validate(legacy))
			require.NoError(t, confmap.Validate(nested))
			selected := nested.effectivePollConfig()
			require.Equal(t, legacy.effectivePollConfig(), selected)
			require.Nil(t, selected.Poll)
			require.Equal(t, 20*time.Second, selected.ControllerConfig.CollectionInterval)
			require.Equal(t, 3*time.Second, selected.ControllerConfig.InitialDelay)
			require.Equal(t, 7*time.Second, selected.ControllerConfig.Timeout)
		})
	}
}

func TestPollConfigPreservesDefaults(t *testing.T) {
	nested := pollTestLoadConfig(t, map[string]any{"poll": map[string]any{"metrics": pollTestMetrics()}})
	legacy := pollTestLoadConfig(t, map[string]any{"metrics": pollTestMetrics()})
	require.NoError(t, confmap.Validate(nested))
	require.Equal(t, legacy.effectivePollConfig(), nested.effectivePollConfig())
	defaults := defaultPollConfig()
	require.Equal(t, defaults.ControllerConfig, nested.Poll.ControllerConfig)
	require.Equal(t, defaultEndpoint, nested.Poll.Endpoint)
	require.Equal(t, defaultVersion, nested.Poll.Version)
	require.Equal(t, defaultCommunity, nested.Poll.Community)
	require.Equal(t, defaultSecurityLevel, nested.Poll.SecurityLevel)
	require.Equal(t, defaultAuthType, nested.Poll.AuthType)
	require.Equal(t, defaultPrivacyType, nested.Poll.PrivacyType)
}

func TestPollConfigRejectsExplicitLegacyOptions(t *testing.T) {
	for _, option := range []struct {
		key              string
		zero, defaultVal any
	}{
		{key: "collection_interval", zero: "0s", defaultVal: "10s"},
		{key: "initial_delay", zero: "0s", defaultVal: "0s"},
		{key: "timeout", zero: "0s", defaultVal: "5s"},
		{key: "endpoint", zero: "", defaultVal: defaultEndpoint},
		{key: "version", zero: "", defaultVal: defaultVersion},
		{key: "community", zero: "", defaultVal: defaultCommunity},
		{key: "user", zero: "", defaultVal: ""},
		{key: "security_level", zero: "", defaultVal: defaultSecurityLevel},
		{key: "auth_type", zero: "", defaultVal: defaultAuthType},
		{key: "auth_password", zero: "", defaultVal: ""},
		{key: "privacy_type", zero: "", defaultVal: defaultPrivacyType},
		{key: "privacy_password", zero: "", defaultVal: ""},
		{key: "resource_attributes", zero: map[string]any{}, defaultVal: nil},
		{key: "attributes", zero: map[string]any{}, defaultVal: nil},
		{key: "metrics", zero: map[string]any{}, defaultVal: nil},
	} {
		t.Run(option.key, func(t *testing.T) {
			for _, value := range []struct {
				name string
				val  any
			}{
				{name: "null"},
				{name: "zero", val: option.zero},
				{name: "default", val: option.defaultVal},
			} {
				t.Run(value.name, func(t *testing.T) {
					cfg := NewFactory().CreateDefaultConfig().(*Config)
					err := confmap.NewFromStringMap(map[string]any{
						"poll": map[string]any{"metrics": pollTestMetrics()}, option.key: value.val,
					}).Unmarshal(cfg)
					require.ErrorContains(t, err, "poll cannot be combined with top-level polling option")
					require.ErrorContains(t, err, option.key)
				})
			}
		})
	}
}

func TestPollConfigRejectsMalformedAndUnknownOptions(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  map[string]any
	}{
		{name: "null poll", raw: map[string]any{"poll": nil}},
		{name: "typed null poll", raw: map[string]any{"poll": map[string]any(nil)}},
		{name: "scalar poll", raw: map[string]any{"poll": "device"}},
		{name: "list poll", raw: map[string]any{"poll": []any{}}},
		{name: "unknown poll option", raw: map[string]any{"poll": map[string]any{"metrics": pollTestMetrics(), "unknown": true}}},
		{name: "unknown top-level option", raw: map[string]any{"poll": map[string]any{"metrics": pollTestMetrics()}, "unknown": true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := NewFactory().CreateDefaultConfig().(*Config)
			require.Error(t, confmap.NewFromStringMap(tc.raw).Unmarshal(cfg))
		})
	}
	cfg := pollTestLoadConfig(t, map[string]any{"poll": map[string]any{}})
	require.ErrorIs(t, confmap.Validate(cfg), errMetricRequired, "an explicit empty poll block cannot silently disable polling")
}

func TestPollConfigValidatesSelectedPollingOptions(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(map[string]any)
		want   string
	}{
		{name: "endpoint", change: func(p map[string]any) { p["endpoint"] = "http://localhost:161" }, want: errEndpointBadScheme.Error()},
		{name: "version", change: func(p map[string]any) { p["version"] = "unknown" }, want: errBadVersion.Error()},
		{name: "v3 user", change: func(p map[string]any) { p["version"], p["user"] = "v3", "" }, want: errEmptyUser.Error()},
		{name: "collection interval", change: func(p map[string]any) { p["collection_interval"] = "-1s" }, want: "collection_interval"},
		{name: "metric mapping", change: func(p map[string]any) {
			p["metrics"] = map[string]any{"device.uptime": map[string]any{
				"unit": "1", "gauge": map[string]any{"value_type": "invalid"},
				"scalar_oids": []any{map[string]any{"oid": pollTestUptimeOID}},
			}}
		}, want: "value_type"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			poll := pollTestOptions("v2c")
			tc.change(poll)
			cfg := pollTestLoadConfig(t, map[string]any{"poll": poll})
			require.ErrorContains(t, confmap.Validate(cfg), tc.want)
		})
	}
}

func TestPollFactoryWarnings(t *testing.T) {
	for _, tc := range []struct {
		name   string
		legacy bool
	}{
		{name: "legacy metrics", legacy: true},
		{name: "nested metrics"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := pollTestOptions("v2c")
			if !tc.legacy {
				raw = map[string]any{"poll": raw}
			}
			cfg := pollTestLoadConfig(t, raw)
			before := pollTestLoadConfig(t, raw)
			require.NoError(t, confmap.Validate(cfg))
			core, observed := observer.New(zap.WarnLevel)
			settings := receivertest.NewNopSettings(metadata.Type)
			settings.Logger = zap.New(core)
			metrics, err := NewFactory().CreateMetrics(t.Context(), settings, cfg, consumertest.NewNop())
			require.NoError(t, err)
			require.NoError(t, metrics.Shutdown(t.Context()))
			if tc.legacy {
				require.Len(t, observed.All(), 1)
				require.Equal(t, zap.WarnLevel, observed.All()[0].Level)
				require.Equal(t, legacyPollingConfigWarning, observed.All()[0].Message)
			} else {
				require.Zero(t, observed.Len())
			}
			require.Equal(t, before, cfg, "metrics construction must not mutate configuration")
		})
	}
}

func TestPollFactoryFailureDoesNotWarn(t *testing.T) {
	for _, nested := range []bool{false, true} {
		name := "legacy"
		if nested {
			name = "nested"
		}
		t.Run(name, func(t *testing.T) {
			raw := pollTestOptions("v2c")
			raw["endpoint"] = "http://localhost:161"
			if nested {
				raw = map[string]any{"poll": raw}
			}
			cfg := pollTestLoadConfig(t, raw)
			core, observed := observer.New(zap.WarnLevel)
			settings := receivertest.NewNopSettings(metadata.Type)
			settings.Logger = zap.New(core)
			_, err := NewFactory().CreateMetrics(t.Context(), settings, cfg, consumertest.NewNop())
			require.ErrorIs(t, err, errEndpointBadScheme)
			require.Zero(t, observed.Len(), "a failed metrics factory must not emit a migration warning")
		})
	}
}

func TestPollFactoryDefaultingDoesNotMutateSource(t *testing.T) {
	for _, nested := range []bool{false, true} {
		name := "legacy"
		if nested {
			name = "nested"
		}
		t.Run(name, func(t *testing.T) {
			raw := map[string]any{
				"endpoint": "localhost",
				"metrics": map[string]any{
					"device.gauge": map[string]any{"gauge": map[string]any{}, "scalar_oids": []any{map[string]any{"oid": pollTestUptimeOID}}},
					"device.sum":   map[string]any{"sum": map[string]any{"monotonic": true}, "scalar_oids": []any{map[string]any{"oid": pollTestUptimeOID}}},
				},
			}
			if nested {
				raw = map[string]any{"poll": raw}
			}
			cfg := pollTestLoadConfig(t, raw)
			before := pollTestLoadConfig(t, raw)
			metrics, err := NewFactory().CreateMetrics(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
			require.NoError(t, err)
			require.NoError(t, metrics.Shutdown(t.Context()))
			require.Equal(t, before, cfg, "endpoint and nested gauge/sum defaults must only change the effective copy")
			selected := cfg.effectivePollConfig()
			require.Equal(t, "localhost", selected.Endpoint)
			require.Empty(t, selected.Metrics["device.gauge"].Unit)
			require.Empty(t, selected.Metrics["device.gauge"].Gauge.ValueType)
			require.Empty(t, selected.Metrics["device.sum"].Sum.ValueType)
			require.Empty(t, selected.Metrics["device.sum"].Sum.Aggregation)
		})
	}
}

func pollTestLoadConfig(t *testing.T, raw map[string]any) *Config {
	t.Helper()
	cfg := NewFactory().CreateDefaultConfig().(*Config)
	require.NoError(t, confmap.NewFromStringMap(raw).Unmarshal(cfg))
	return cfg
}

func pollTestMetrics() map[string]any {
	return map[string]any{"device.uptime": map[string]any{
		"unit": "1", "gauge": map[string]any{"value_type": "int"},
		"scalar_oids": []any{map[string]any{"oid": pollTestUptimeOID}},
	}}
}

func pollTestOptions(version string) map[string]any {
	options := map[string]any{
		"collection_interval": "20s", "initial_delay": "3s", "timeout": "7s",
		"endpoint": "udp://device.example:161", "version": version, "community": "device-community",
		"attributes": map[string]any{
			"direction": map[string]any{"enum": []string{"in", "out"}},
			"interface": map[string]any{"oid": "1.3.6.1.2.1.2.2.1.2"},
			"index":     map[string]any{"indexed_value_prefix": "if"},
		},
		"resource_attributes": map[string]any{
			"device":    map[string]any{"scalar_oid": "1.3.6.1.2.1.1.5.0"},
			"interface": map[string]any{"oid": "1.3.6.1.2.1.2.2.1.2"},
			"index":     map[string]any{"indexed_value_prefix": "interface"},
		},
		"metrics": map[string]any{
			"device.uptime": map[string]any{
				"description": "Device uptime", "unit": "1", "gauge": map[string]any{"value_type": "int"},
				"scalar_oids": []any{map[string]any{
					"oid": pollTestUptimeOID, "resource_attributes": []string{"device"},
					"attributes": []any{map[string]any{"name": "direction", "value": "in"}},
				}},
			},
			"device.octets": map[string]any{
				"description": "Interface octets", "unit": "By",
				"sum": map[string]any{"value_type": "int", "aggregation": "cumulative", "monotonic": true},
				"column_oids": []any{map[string]any{
					"oid": "1.3.6.1.2.1.2.2.1.10", "resource_attributes": []string{"interface", "index"},
					"attributes": []any{map[string]any{"name": "interface"}, map[string]any{"name": "index"}, map[string]any{"name": "direction", "value": "in"}},
				}},
			},
		},
	}
	if version == "v3" {
		options["user"] = "otel"
		options["security_level"] = "auth_priv"
		options["auth_type"] = "SHA256"
		options["auth_password"] = "auth-password"
		options["privacy_type"] = "AES"
		options["privacy_password"] = "privacy-password"
	}
	return options
}
