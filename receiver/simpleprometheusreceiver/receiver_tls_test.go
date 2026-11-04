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
// server name are forwarded to the underlying prometheus receiver and survive
// the reload that the prometheus receiver runs. They are valid
// confighttp.ClientConfig TLS options; dropping them silently turns a TLS
// scrape into a misconfigured one with no error.
func TestGetPrometheusConfigInlineTLS(t *testing.T) {
	caPem, err := os.ReadFile("./testdata/test_cert.pem")
	require.NoError(t, err)
	certPem, keyPem := generateSelfSignedPEM(t)

	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = "localhost:1234"
	clientConfig.TLS = configtls.ClientConfig{
		Config: configtls.Config{
			CAPem:   configopaque.String(caPem),
			CertPem: configopaque.String(certPem),
			KeyPem:  configopaque.String(keyPem),
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
	require.NoError(t, got.PrometheusConfig.Reload())

	tlsCfg := got.PrometheusConfig.ScrapeConfigs[0].HTTPClientConfig.TLSConfig
	assert.Equal(t, string(caPem), tlsCfg.CA, "inline CA PEM (ca_pem) must be forwarded to the prometheus scrape config")
	assert.Equal(t, string(certPem), tlsCfg.Cert, "cert_pem must be forwarded to the prometheus scrape config")
	assert.Equal(t, configutil.Secret(keyPem), tlsCfg.Key, "key_pem must be forwarded to the prometheus scrape config")
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
			wantKeys: []any{
				"cipher_suites", "include_insecure_cipher_suites", "curve_preferences",
				"include_system_ca_certs_pool", "tpm",
			},
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

// TestTLSSettingsAreClassified fails when configtls.ClientConfig gains a
// setting that this receiver does not handle, so the setting is not silently
// dropped.
func TestTLSSettingsAreClassified(t *testing.T) {
	handled := map[string]string{
		// Forwarded to the prometheus scrape TLS config.
		"ca_file":              "forwarded",
		"ca_pem":               "forwarded",
		"cert_file":            "forwarded",
		"cert_pem":             "forwarded",
		"key_file":             "forwarded",
		"key_pem":              "forwarded",
		"server_name_override": "forwarded",
		"insecure_skip_verify": "forwarded",
		"min_version":          "forwarded",
		"max_version":          "forwarded",
		// No prometheus equivalent; named in the unsupportedTLSSettings warning.
		"cipher_suites":                  "warned",
		"include_insecure_cipher_suites": "warned",
		"curve_preferences":              "warned",
		"include_system_ca_certs_pool":   "warned",
		"tpm":                            "warned",
		// Prometheus re-reads changed certificate, key and CA files itself.
		"reload_interval": "covered by prometheus",
		// Selects between an http and an https scrape.
		"insecure": "controls scheme",
	}

	keys := configKeys(reflect.TypeFor[configtls.ClientConfig]())
	for _, key := range keys {
		if _, ok := handled[key]; !ok {
			t.Errorf("tls key %q is not handled: forward it in getPrometheusConfig or add it to "+
				"unsupportedTLSSettings, update the tls section of README.md, then add it here", key)
		}
	}
	for key := range handled {
		if !slices.Contains(keys, key) {
			t.Errorf("tls key %q is listed here but configtls.ClientConfig no longer has it", key)
		}
	}
}

// configKeys returns the config keys of the struct type typ: the mapstructure
// tag name, or the field name when the tag has none, with squashed fields
// flattened into the parent and other unexported fields skipped.
func configKeys(typ reflect.Type) []string {
	var keys []string
	for field := range typ.Fields() {
		name, opts, _ := strings.Cut(field.Tag.Get("mapstructure"), ",")
		if strings.Contains(opts, "squash") {
			keys = append(keys, configKeys(field.Type)...)
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
