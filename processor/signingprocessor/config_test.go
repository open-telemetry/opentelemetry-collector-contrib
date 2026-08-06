// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signingprocessor

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
)

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       component.ID
		expected *Config
	}{
		{
			id: component.NewID(component.MustNewType("signing")),
			expected: &Config{
				Algorithm: "RS256",
				CertificateRef: "fingerprint",
				KeySource: KeySourceConfig{
					Type: KeySourceFile,
					File: &FileKeyConfig{
						CertFile: "/etc/otelcol/signing-cert.pem",
						KeyFile:  "/etc/otelcol/signing-key.pem",
					},
				},
			},
		},
		{
			id: component.NewIDWithName(component.MustNewType("signing"), "sha512_full"),
			expected: &Config{
				Algorithm: "RS512",
				CertificateRef: "full",
				KeySource: KeySourceConfig{
					Type: KeySourceFile,
					File: &FileKeyConfig{
						CertFile: "/etc/otelcol/signing-cert.pem",
						KeyFile:  "/etc/otelcol/signing-key.pem",
					},
				},
			},
		},
		{
			id: component.NewIDWithName(component.MustNewType("signing"), "env"),
			expected: &Config{
				Algorithm: "RS256",
				CertificateRef: "fingerprint",
				KeySource: KeySourceConfig{
					Type: KeySourceEnv,
					Env: &EnvKeyConfig{
						CertEnvVar: "SIGNING_CERT_PEM",
						KeyEnvVar:  "SIGNING_KEY_PEM",
					},
				},
			},
		},
		{
			id: component.NewIDWithName(component.MustNewType("signing"), "k8s"),
			expected: &Config{
				Algorithm: "RS256",
				CertificateRef: "fingerprint",
				KeySource: KeySourceConfig{
					Type: KeySourceK8sSecret,
					K8sSecret: &K8sSecretConfig{
						Name:      "signing-secret",
						Namespace: "default",
						CertKey:   "tls.crt",
						KeyKey:    "tls.key",
					},
				},
			},
		},
		{
			id: component.NewIDWithName(component.MustNewType("signing"), "bao"),
			expected: &Config{
				Algorithm: "RS256",
				CertificateRef: "fingerprint",
				KeySource: KeySourceConfig{
					Type: KeySourceBao,
					Bao: &BaoKeyConfig{
						Address:    "https://bao.example.com",
						SecretPath: "secret/data/signing",
						CertField:  "certificate",
						KeyField:   "private_key",
					},
				},
			},
		},
		{
			id: component.NewIDWithName(component.MustNewType("signing"), "hmac"),
			expected: &Config{
				Algorithm:      "HMAC-SHA256",
				CertificateRef: "fingerprint",
				KeySource: KeySourceConfig{
					Type: KeySourceEnv,
					Env:  &EnvKeyConfig{HMACKeyEnvVar: "SIGNING_HMAC_KEY"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestLoadConfigInvalid(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config_invalid.yaml"))
	require.NoError(t, err)

	invalidIDs := []string{
		"signing/missing_key_source",
		"signing/bad_hash",
		"signing/bad_cert_ref",
		"signing/file_missing_cert",
		"signing/env_missing_key",
		"signing/k8s_missing_name",
		"signing/bao_missing_path",
	}

	for _, id := range invalidIDs {
		t.Run(id, func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(id)
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.Error(t, cfg.(*Config).Validate(), "expected Validate() to fail for %s", id)
		})
	}
}
