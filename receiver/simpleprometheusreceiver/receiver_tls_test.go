// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package simpleprometheusreceiver

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	configutil "github.com/prometheus/common/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/simpleprometheusreceiver/internal/metadata"
)

// TestGetPrometheusConfigInlineTLS proves that inline PEM TLS material and the
// server name are forwarded to the underlying prometheus receiver. Both are
// valid confighttp.ClientConfig TLS options; dropping them silently turns a
// TLS scrape into a misconfigured one with no error.
func TestGetPrometheusConfigInlineTLS(t *testing.T) {
	caPem, err := os.ReadFile("./testdata/test_cert.pem")
	require.NoError(t, err)

	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = "localhost:1234"
	clientConfig.TLS = configtls.ClientConfig{
		Config: configtls.Config{
			CAPem: configopaque.String(caPem),
		},
		ServerName: "example.com",
	}

	cfg := &Config{
		ClientConfig:       clientConfig,
		CollectionInterval: 10 * time.Second,
		MetricsPath:        "/metrics",
	}

	got, err := getPrometheusConfig(cfg, zap.NewNop())
	require.NoError(t, err)

	tlsCfg := got.PrometheusConfig.ScrapeConfigs[0].HTTPClientConfig.TLSConfig
	assert.Equal(t, string(caPem), tlsCfg.CA, "inline CA PEM (ca_pem) must be forwarded to the prometheus scrape config")
	assert.Equal(t, "example.com", tlsCfg.ServerName, "server_name must be forwarded to the prometheus scrape config")
}

// TestGetPrometheusConfigTLSVersions proves that the resolved min_version and
// max_version are forwarded to the prometheus scrape config, so the defaults
// match other collector HTTP clients, and survive the reload that the
// prometheus receiver runs.
func TestGetPrometheusConfigTLSVersions(t *testing.T) {
	tests := []struct {
		name                   string
		minVersion, maxVersion string
		wantMin, wantMax       configutil.TLSVersion
		wantValidateErr        string
		wantErr                string
	}{
		{name: "unset uses collector defaults", wantMin: configutil.TLSVersions["TLS12"]},
		{name: "min 1.3", minVersion: "1.3", wantMin: configutil.TLSVersions["TLS13"]},
		{
			name: "min 1.0 max 1.2", minVersion: "1.0", maxVersion: "1.2",
			wantMin: configutil.TLSVersions["TLS10"], wantMax: configutil.TLSVersions["TLS12"],
		},
		{
			name: "max only", maxVersion: "1.2",
			wantMin: configutil.TLSVersions["TLS12"], wantMax: configutil.TLSVersions["TLS12"],
		},
		{
			name: "unknown version", minVersion: "1.4",
			wantValidateErr: `invalid TLS min_version: unsupported TLS version: "1.4"`,
			wantErr:         `unsupported TLS version: "1.4"`,
		},
		{
			name: "min greater than max", minVersion: "1.3", maxVersion: "1.2",
			wantValidateErr: "invalid TLS configuration: min_version cannot be greater than max_version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientConfig := confighttp.NewDefaultClientConfig()
			clientConfig.Endpoint = "localhost:1234"
			clientConfig.TLS = configtls.ClientConfig{
				Config: configtls.Config{
					MinVersion: tt.minVersion,
					MaxVersion: tt.maxVersion,
				},
			}

			cfg := &Config{
				ClientConfig:       clientConfig,
				CollectionInterval: 10 * time.Second,
				MetricsPath:        "/metrics",
			}

			// The collector validates the full config before the receiver
			// starts; that is where an inconsistent min/max pair is rejected.
			if err := confmap.Validate(cfg); tt.wantValidateErr != "" {
				require.ErrorContains(t, err, tt.wantValidateErr)
				if tt.wantErr == "" {
					return
				}
			} else {
				require.NoError(t, err)
			}

			got, err := getPrometheusConfig(cfg, zap.NewNop())
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.NoError(t, got.PrometheusConfig.Reload())

			tlsCfg := got.PrometheusConfig.ScrapeConfigs[0].HTTPClientConfig.TLSConfig
			assert.Equal(t, tt.wantMin, tlsCfg.MinVersion)
			assert.Equal(t, tt.wantMax, tlsCfg.MaxVersion)
		})
	}
}

// TestStartInlineTLS runs the forwarded TLS settings through the real
// receiver Start path, which re-marshals the prometheus config. Inspecting
// getPrometheusConfig output alone does not catch failures on that reload.
func TestStartInlineTLS(t *testing.T) {
	caPem, err := os.ReadFile("./testdata/test_cert.pem")
	require.NoError(t, err)
	certPem, keyPem := generateSelfSignedPEM(t)

	tests := []struct {
		name string
		tls  configtls.ClientConfig
	}{
		{
			name: "ca_pem",
			tls: configtls.ClientConfig{
				Config: configtls.Config{CAPem: configopaque.String(caPem)},
			},
		},
		{
			name: "server_name_override",
			tls: configtls.ClientConfig{
				ServerName: "example.com",
			},
		},
		{
			name: "cert_pem and key_pem",
			tls: configtls.ClientConfig{
				Config: configtls.Config{
					CertPem: configopaque.String(certPem),
					KeyPem:  configopaque.String(keyPem),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.ClientConfig.Endpoint = "localhost:1234"
			cfg.ClientConfig.TLS = tt.tls

			r, err := NewFactory().CreateMetrics(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
			require.NoError(t, err)
			require.NoError(t, r.Start(t.Context(), componenttest.NewNopHost()))
			require.NoError(t, r.Shutdown(t.Context()))
		})
	}
}

func generateSelfSignedPEM(t *testing.T) (certPem, keyPem []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	keyDer, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)

	certPem = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPem = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDer})
	return certPem, keyPem
}

// TestStartWarnsUnsupportedTLSSettings checks that tls settings the prometheus
// scrape config cannot express are named in a warning when the receiver starts.
func TestStartWarnsUnsupportedTLSSettings(t *testing.T) {
	caPem, err := os.ReadFile("./testdata/test_cert.pem")
	require.NoError(t, err)

	tests := []struct {
		name     string
		tls      configtls.ClientConfig
		wantKeys []any
	}{
		{
			name: "forwarded settings only",
			tls: configtls.ClientConfig{
				Config:     configtls.Config{CAPem: configopaque.String(caPem), MinVersion: "1.3"},
				ServerName: "example.com",
			},
		},
		{
			name: "system pool without a CA",
			tls: configtls.ClientConfig{
				Insecure: false,
				Config:   configtls.Config{IncludeSystemCACertsPool: true},
			},
		},
		{
			name: "insecure http ignores all tls settings",
			tls: configtls.ClientConfig{
				Insecure: true,
				Config:   configtls.Config{CipherSuites: []string{"TLS_AES_128_GCM_SHA256"}},
			},
		},
		{
			name: "reload_interval is covered by prometheus file reloads",
			tls: configtls.ClientConfig{
				Config: configtls.Config{CAPem: configopaque.String(caPem), ReloadInterval: time.Minute},
			},
		},
		{
			name: "insecure cipher suites without cipher_suites",
			tls: configtls.ClientConfig{
				Config: configtls.Config{CAPem: configopaque.String(caPem), IncludeInsecureCipherSuites: true},
			},
		},
		{
			name: "tpm",
			tls: configtls.ClientConfig{
				Config: configtls.Config{CAPem: configopaque.String(caPem), TPMConfig: configtls.TPMConfig{Enabled: true}},
			},
			wantKeys: []any{"tpm"},
		},
		{
			name: "every unsupported setting",
			tls: configtls.ClientConfig{
				Config: configtls.Config{
					CAPem:                       configopaque.String(caPem),
					CipherSuites:                []string{"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256"},
					IncludeInsecureCipherSuites: true,
					CurvePreferences:            []string{"X25519"},
					IncludeSystemCACertsPool:    true,
					ReloadInterval:              time.Minute,
					TPMConfig:                   configtls.TPMConfig{Enabled: true},
				},
			},
			wantKeys: func() []any {
				keys := make([]any, 0, len(warnedUnsupportedTLSKeys))
				for _, key := range warnedUnsupportedTLSKeys {
					keys = append(keys, key)
				}
				return keys
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.ClientConfig.Endpoint = "localhost:1234"
			cfg.ClientConfig.TLS = tt.tls

			core, logs := observer.New(zapcore.WarnLevel)
			set := receivertest.NewNopSettings(metadata.Type)
			set.Logger = zap.New(core)
			r, err := NewFactory().CreateMetrics(t.Context(), set, cfg, consumertest.NewNop())
			require.NoError(t, err)
			require.NoError(t, r.Start(t.Context(), componenttest.NewNopHost()))
			require.NoError(t, r.Shutdown(t.Context()))

			warned := logs.FilterMessageSnippet("tls settings have no equivalent").All()
			if tt.wantKeys == nil {
				assert.Empty(t, warned)
				return
			}
			require.Len(t, warned, 1)
			assert.Equal(t, tt.wantKeys, warned[0].ContextMap()["settings"])
		})
	}
}

// The tls keys of configtls.ClientConfig, by how this receiver handles them.
// TestTLSSettingsAreClassified checks that every key is in exactly one set,
// and TestForwardedTLSSettingsSurviveReload checks every forwarded key.
var (
	// forwardedTLSKeys reach the prometheus scrape TLS config.
	forwardedTLSKeys = []string{
		"ca_file", "ca_pem", "cert_file", "cert_pem", "key_file", "key_pem",
		"server_name_override", "insecure_skip_verify", "min_version", "max_version",
	}
	// warnedUnsupportedTLSKeys have no prometheus equivalent and are named in
	// the start-time warning from unsupportedTLSSettings, in the same order.
	warnedUnsupportedTLSKeys = []string{
		"cipher_suites", "include_insecure_cipher_suites", "curve_preferences",
		"include_system_ca_certs_pool", "tpm",
	}
	// coveredByPrometheusTLSKeys are not forwarded or warned about because
	// prometheus already provides the behavior: it re-reads changed
	// certificate, key and CA files, which is what reload_interval does.
	coveredByPrometheusTLSKeys = []string{"reload_interval"}
	// controlsSchemeTLSKeys select between an http and an https scrape.
	controlsSchemeTLSKeys = []string{"insecure"}
)

// TestTLSSettingsAreClassified fails when configtls.ClientConfig gains a
// setting that this receiver does not handle.
func TestTLSSettingsAreClassified(t *testing.T) {
	classes := map[string][]string{
		"forwardedTLSKeys":           forwardedTLSKeys,
		"warnedUnsupportedTLSKeys":   warnedUnsupportedTLSKeys,
		"coveredByPrometheusTLSKeys": coveredByPrometheusTLSKeys,
		"controlsSchemeTLSKeys":      controlsSchemeTLSKeys,
	}
	classOf := map[string]string{}
	for class, keys := range classes {
		for _, key := range keys {
			if other, ok := classOf[key]; ok {
				t.Fatalf("tls key %q is in both %s and %s", key, other, class)
			}
			classOf[key] = class
		}
	}

	keys := configKeys(reflect.TypeFor[configtls.ClientConfig]())
	for _, key := range keys {
		if _, ok := classOf[key]; !ok {
			t.Errorf("tls key %q is not classified: forward it in getPrometheusConfig or add it to "+
				"the warning list in unsupportedTLSSettings (receiver.go), update the tls section "+
				"of README.md, then classify it in this test", key)
		}
	}
	for key, class := range classOf {
		if !slices.Contains(keys, key) {
			t.Errorf("%s lists tls key %q, which configtls.ClientConfig no longer has", class, key)
		}
	}
}

// configKeys returns the config keys that confmap decodes into the struct
// type typ. It follows mapstructure: the key is the name in the mapstructure
// tag, or the exact field name when the tag has no name, fields with the
// squash option are flattened into the parent, and other unexported fields
// are skipped.
func configKeys(typ reflect.Type) []string {
	var keys []string
	for field := range typ.Fields() {
		name, opts, _ := strings.Cut(field.Tag.Get("mapstructure"), ",")
		if slices.Contains(strings.Split(opts, ","), "squash") {
			fieldType := field.Type
			if fieldType.Kind() == reflect.Pointer {
				fieldType = fieldType.Elem()
			}
			keys = append(keys, configKeys(fieldType)...)
			continue
		}
		if !field.IsExported() {
			continue
		}
		if name == "" {
			name = field.Name
		}
		keys = append(keys, name)
	}
	return keys
}

// TestConfigKeysMatchConfmap checks that configKeys names the same keys that
// confmap accepts, including for untagged fields.
func TestConfigKeysMatchConfmap(t *testing.T) {
	type inner struct {
		Inner string `mapstructure:"inner"`
	}
	type sample struct {
		Tagged    string `mapstructure:"tagged"`
		OmitEmpty string `mapstructure:",omitempty"`
		Untagged  string
		inner     `mapstructure:",squash"`
		Nested    inner
		hidden    string
	}

	keys := configKeys(reflect.TypeFor[sample]())
	require.Equal(t, []string{"tagged", "OmitEmpty", "Untagged", "inner", "Nested"}, keys)

	raw := map[string]any{}
	for _, key := range keys {
		raw[key] = "x"
	}
	raw["Nested"] = map[string]any{"inner": "x"}
	var got sample
	require.NoError(t, confmap.NewFromStringMap(raw).Unmarshal(&got))
	assert.Equal(t, sample{
		Tagged: "x", OmitEmpty: "x", Untagged: "x",
		inner: inner{Inner: "x"}, Nested: inner{Inner: "x"},
	}, got)

	// Keys are matched case-sensitively, and unexported fields are not keys.
	for _, key := range []string{"untagged", "hidden"} {
		var s sample
		assert.Error(t, confmap.NewFromStringMap(map[string]any{key: "x"}).Unmarshal(&s), key)
		assert.Empty(t, s.hidden)
	}
}

// TestForwardedTLSSettingsSurviveReload checks that every forwarded tls key
// reaches the prometheus scrape TLS config after the reload that the
// prometheus receiver runs on start.
func TestForwardedTLSSettingsSurviveReload(t *testing.T) {
	caPem, err := os.ReadFile("./testdata/test_cert.pem")
	require.NoError(t, err)
	certPem, keyPem := generateSelfSignedPEM(t)

	dir := t.TempDir()
	caFile := filepath.Join(dir, "ca.pem")
	certFile := filepath.Join(dir, "cert.pem")
	keyFile := filepath.Join(dir, "key.pem")
	require.NoError(t, os.WriteFile(caFile, caPem, 0o600))
	require.NoError(t, os.WriteFile(certFile, certPem, 0o600))
	require.NoError(t, os.WriteFile(keyFile, keyPem, 0o600))

	// Every case sets insecure to false so the scrape uses https and the tls
	// settings are forwarded.
	tests := []struct {
		// keys are the tls keys that the case sets.
		keys []string
		cfg  configtls.Config
		// serverName and insecureSkipVerify are configtls.ClientConfig fields.
		serverName         string
		insecureSkipVerify bool
		want               configutil.TLSConfig
	}{
		{
			keys: []string{"ca_file"},
			cfg:  configtls.Config{CAFile: caFile},
			want: configutil.TLSConfig{CAFile: caFile},
		},
		{
			keys: []string{"ca_pem"},
			cfg:  configtls.Config{CAPem: configopaque.String(caPem)},
			want: configutil.TLSConfig{CA: string(caPem)},
		},
		{
			keys: []string{"cert_file", "key_file"},
			cfg:  configtls.Config{CertFile: certFile, KeyFile: keyFile},
			want: configutil.TLSConfig{CertFile: certFile, KeyFile: keyFile},
		},
		{
			keys: []string{"cert_pem", "key_pem"},
			cfg:  configtls.Config{CertPem: configopaque.String(certPem), KeyPem: configopaque.String(keyPem)},
			want: configutil.TLSConfig{Cert: string(certPem), Key: configutil.Secret(keyPem)},
		},
		{
			keys: []string{"cert_file", "key_pem"},
			cfg:  configtls.Config{CertFile: certFile, KeyPem: configopaque.String(keyPem)},
			want: configutil.TLSConfig{CertFile: certFile, Key: configutil.Secret(keyPem)},
		},
		{
			keys:       []string{"server_name_override"},
			serverName: "example.com",
			want:       configutil.TLSConfig{ServerName: "example.com"},
		},
		{
			keys:               []string{"insecure_skip_verify"},
			insecureSkipVerify: true,
			want:               configutil.TLSConfig{InsecureSkipVerify: true},
		},
		{
			keys: []string{"min_version", "max_version"},
			cfg:  configtls.Config{MinVersion: "1.1", MaxVersion: "1.2"},
			want: configutil.TLSConfig{MinVersion: configutil.TLSVersions["TLS11"], MaxVersion: configutil.TLSVersions["TLS12"]},
		},
	}

	// The cases must cover exactly the forwarded keys.
	var covered []string
	for _, tt := range tests {
		for _, key := range tt.keys {
			if !slices.Contains(forwardedTLSKeys, key) {
				t.Errorf("case %v sets tls key %q, which is not in forwardedTLSKeys", tt.keys, key)
			}
		}
		covered = append(covered, tt.keys...)
	}
	for _, key := range forwardedTLSKeys {
		if !slices.Contains(covered, key) {
			t.Errorf("forwarded tls key %q has no case in this test", key)
		}
	}

	for _, tt := range tests {
		t.Run(strings.Join(tt.keys, "+"), func(t *testing.T) {
			clientConfig := confighttp.NewDefaultClientConfig()
			clientConfig.Endpoint = "localhost:1234"
			clientConfig.TLS = configtls.ClientConfig{
				Config:             tt.cfg,
				Insecure:           false,
				InsecureSkipVerify: tt.insecureSkipVerify,
				ServerName:         tt.serverName,
			}
			cfg := &Config{
				ClientConfig:       clientConfig,
				CollectionInterval: 10 * time.Second,
				MetricsPath:        "/metrics",
			}
			require.NoError(t, confmap.Validate(cfg))

			got, err := getPrometheusConfig(cfg, zap.NewNop())
			require.NoError(t, err)
			require.NoError(t, got.PrometheusConfig.Reload())

			scrapeConfig := got.PrometheusConfig.ScrapeConfigs[0]
			assert.Equal(t, "https", scrapeConfig.Scheme)
			want := tt.want
			if want.MinVersion == 0 {
				// An unset min_version resolves to the collector default.
				want.MinVersion = configutil.TLSVersions["TLS12"]
			}
			assert.Equal(t, want, scrapeConfig.HTTPClientConfig.TLSConfig)
		})
	}
}
