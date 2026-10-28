// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clientauth // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/googleclientauthextension/internal/clientauth"

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"cloud.google.com/go/auth"
	"cloud.google.com/go/auth/credentials/idtoken"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension"
)

type mockTokenProvider struct {
	token string
}

func (tp *mockTokenProvider) Token(context.Context) (*auth.Token, error) {
	return &auth.Token{
		Value: tp.token,
	}, nil
}

func TestCreateDefaultConfig(t *testing.T) {
	cfg := CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateExtension(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "testdata/fake_creds.json")
	ext, err := CreateExtension(t.Context(), extension.Settings{}, CreateDefaultConfig())
	assert.NotNil(t, ext)
	assert.NoError(t, err)
}

func TestStart(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "testdata/fake_creds.json")
	ext, err := CreateExtension(t.Context(), extension.Settings{}, CreateDefaultConfig())
	assert.NotNil(t, ext)
	assert.NoError(t, err)
	err = ext.Start(t.Context(), nil)
	assert.NoError(t, err)
}

func TestStart_WithError(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "testdata/foo.json")
	ext, err := CreateExtension(t.Context(), extension.Settings{}, CreateDefaultConfig())
	assert.NotNil(t, ext)
	assert.NoError(t, err)
	err = ext.Start(t.Context(), nil)
	assert.Error(t, err)
}

func TestStart_WithNoProjectError(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "testdata/fake_creds_no_project.json")
	t.Setenv("GOOGLE_CLOUD_PROJECT", "")
	ext, err := CreateExtension(t.Context(), extension.Settings{}, CreateDefaultConfig())
	assert.NotNil(t, ext)
	assert.NoError(t, err)
	err = ext.Start(t.Context(), nil)
	assert.Error(t, err)
}

func TestStart_WithProjectFromEnvVar(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "testdata/fake_creds_no_project.json")
	t.Setenv("GOOGLE_CLOUD_PROJECT", "my-project")
	ext, err := CreateExtension(t.Context(), extension.Settings{}, CreateDefaultConfig())
	assert.NotNil(t, ext)
	assert.NoError(t, err)
	err = ext.Start(t.Context(), nil)
	assert.NoError(t, err)
}

func TestStart_WithProjectOverride(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "testdata/fake_creds.json")
	t.Setenv("GOOGLE_CLOUD_PROJECT", "my-overridden-project")
	ext, err := CreateExtension(t.Context(), extension.Settings{}, CreateDefaultConfig())
	assert.NotNil(t, ext)
	assert.NoError(t, err)
	err = ext.Start(t.Context(), nil)
	assert.NoError(t, err)
	// verify the value of the overridden project id
	ca, ok := ext.(*clientAuthenticator)
	if !ok {
		t.Fatalf("Returned extension is not of type *clientAuthenticator. Got: %T", ext)
	}
	assert.Equal(t, "my-overridden-project", ca.config.Project)
}

func TestStart_idtoken(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "testdata/fake_creds.json")
	ca := &clientAuthenticator{
		config: &Config{
			Project: "my-project",
			Scopes: []string{
				"https://www.googleapis.com/auth/cloud-platform",
				"https://www.googleapis.com/auth/logging.write",
				"https://www.googleapis.com/auth/monitoring.write",
				"https://www.googleapis.com/auth/trace.append",
			},
			TokenType: idToken,
			Audience:  "my-audience",
		},
		newIDTokenCredentials: func(opts *idtoken.Options) (*auth.Credentials, error) {
			assert.Equal(t, "my-audience", opts.Audience)
			assert.NotEmpty(t, opts.CredentialsJSON)

			return auth.NewCredentials(&auth.CredentialsOptions{
				TokenProvider: &mockTokenProvider{token: "dummy token"},
			}), nil
		},
	}
	err := ca.Start(t.Context(), nil)
	assert.NoError(t, err)

	token, err := ca.Token()
	assert.NoError(t, err)
	assert.Equal(t, "dummy token", token.AccessToken)
}

func Test_newTokenSource_idtokenWithoutCredsJSON(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "")
	ca := &clientAuthenticator{
		config: &Config{
			Project: "my-project",
			Scopes: []string{
				"https://www.googleapis.com/auth/cloud-platform",
				"https://www.googleapis.com/auth/logging.write",
				"https://www.googleapis.com/auth/monitoring.write",
				"https://www.googleapis.com/auth/trace.append",
			},
			TokenType: idToken,
			Audience:  "my-audience",
		},
		newIDTokenCredentials: func(opts *idtoken.Options) (*auth.Credentials, error) {
			assert.Equal(t, "my-audience", opts.Audience)
			assert.Empty(t, opts.CredentialsJSON)

			return auth.NewCredentials(&auth.CredentialsOptions{
				TokenProvider: &mockTokenProvider{token: "dummy token"},
			}), nil
		},
	}
	ts, err := ca.newTokenSource(auth.NewCredentials(&auth.CredentialsOptions{
		ProjectIDProvider: auth.CredentialsPropertyFunc(func(context.Context) (string, error) {
			return "my-project", nil
		}),
	}))
	assert.NotNil(t, ts)
	assert.NoError(t, err)

	token, err := ts.Token()
	assert.NoError(t, err)
	assert.Equal(t, "dummy token", token.AccessToken)
}

func TestStart_X509ExternalAccount(t *testing.T) {
	tmpDir := t.TempDir()

	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test-workload"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &privKey.PublicKey, privKey)
	require.NoError(t, err)

	certPath := filepath.Join(tmpDir, "leaf.pem")
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	require.NoError(t, os.WriteFile(certPath, certPEM, 0o600))

	keyPath := filepath.Join(tmpDir, "leaf.key")
	pkcs8Bytes, err := x509.MarshalPKCS8PrivateKey(privKey)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8Bytes})
	require.NoError(t, os.WriteFile(keyPath, keyPEM, 0o600))

	certConfigPath := filepath.Join(tmpDir, "cert_config.json")
	certConfig, err := json.Marshal(map[string]any{
		"cert_configs": map[string]any{
			"workload": map[string]string{
				"cert_path": certPath,
				"key_path":  keyPath,
			},
		},
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(certConfigPath, certConfig, 0o600))

	credConfigPath := filepath.Join(tmpDir, "cred_config.json")
	credConfig, err := json.Marshal(map[string]any{
		"universe_domain":    "googleapis.com",
		"type":               "external_account",
		"audience":           "//iam.googleapis.com/projects/123456/locations/global/workloadIdentityPools/test-pool/providers/test-provider",
		"subject_token_type": "urn:ietf:params:oauth:token-type:mtls",
		"token_url":          "https://sts.mtls.googleapis.com/v1/token",
		"credential_source": map[string]any{
			"certificate": map[string]any{
				"certificate_config_location": certConfigPath,
			},
		},
		"quota_project_id": "my-quota-project",
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(credConfigPath, credConfig, 0o600))

	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", credConfigPath)
	t.Setenv("GOOGLE_CLOUD_PROJECT", "my-project")

	ext, err := CreateExtension(t.Context(), extension.Settings{}, CreateDefaultConfig())
	require.NoError(t, err)
	require.NoError(t, ext.Start(t.Context(), nil))

	ca, ok := ext.(*clientAuthenticator)
	require.True(t, ok)
	assert.Equal(t, "my-project", ca.config.Project)
	assert.Equal(t, "my-quota-project", ca.config.QuotaProject)
}
