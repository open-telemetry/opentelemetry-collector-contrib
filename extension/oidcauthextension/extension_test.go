// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oidcauthextension

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensionauth"
	"go.uber.org/zap"
)

func newTestExtension(t *testing.T, cfg *Config) extension.Extension {
	t.Helper()
	return newExtension(cfg, zap.NewNop())
}

func TestOIDCAuthenticationSucceeded(t *testing.T) {
	// prepare
	oidcServer, err := newOIDCServer()
	require.NoError(t, err)
	oidcServer.Start()
	defer oidcServer.Close()

	config := &Config{
		IssuerURL:   oidcServer.URL,
		Audience:    "unit-test",
		GroupsClaim: "memberships",
	}
	p := newTestExtension(t, config)

	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	payload, _ := json.Marshal(map[string]any{
		"sub":         "jdoe@example.com",
		"name":        "jdoe",
		"iss":         oidcServer.URL,
		"aud":         "unit-test",
		"exp":         time.Now().Add(time.Minute).Unix(),
		"memberships": []string{"department-1", "department-2"},
	})
	token, err := oidcServer.token(payload)
	require.NoError(t, err)

	srvAuth, ok := p.(extensionauth.Server)
	require.True(t, ok)

	// test
	ctx, err := srvAuth.Authenticate(t.Context(), map[string][]string{"authorization": {fmt.Sprintf("Bearer %s", token)}})

	// verify
	assert.NoError(t, err)
	assert.NotNil(t, ctx)

	// test, upper-case header
	ctx, err = srvAuth.Authenticate(t.Context(), map[string][]string{"Authorization": {fmt.Sprintf("Bearer %s", token)}})

	// verify
	assert.NoError(t, err)
	assert.NotNil(t, ctx)

	// TODO(jpkroehling): assert that the authentication routine set the subject/membership to the resource
}

func TestOIDCAuthenticationSucceededMultipleProviders(t *testing.T) {
	// prepare
	oidcServer1, err := newOIDCServer()
	require.NoError(t, err)
	oidcServer1.Start()
	defer oidcServer1.Close()

	oidcServer2, err := newOIDCServer()
	require.NoError(t, err)
	oidcServer2.Start()
	defer oidcServer2.Close()

	config := &Config{
		IssuerURL:   oidcServer1.URL,
		Audience:    "unit-test-1",
		GroupsClaim: "memberships",
		Providers: []ProviderCfg{
			{
				IssuerURL:   oidcServer2.URL,
				Audience:    "unit-test-2",
				GroupsClaim: "groups",
			},
		},
	}
	p := newTestExtension(t, config)

	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	srvAuth, ok := p.(extensionauth.Server)
	require.True(t, ok)

	payload1, _ := json.Marshal(map[string]any{
		"sub":         "jdoe@example.com",
		"name":        "jdoe",
		"iss":         oidcServer1.URL,
		"aud":         "unit-test-1",
		"exp":         time.Now().Add(time.Minute).Unix(),
		"memberships": []string{"department-1", "department-2"},
	})
	token1, err := oidcServer1.token(payload1)
	require.NoError(t, err)

	payload2, _ := json.Marshal(map[string]any{
		"sub":    "jdough@example.com",
		"name":   "jdough",
		"iss":    oidcServer2.URL,
		"aud":    "unit-test-2",
		"exp":    time.Now().Add(time.Minute).Unix(),
		"groups": []string{"department-1", "department-2"},
	})
	token2, err := oidcServer2.token(payload2)
	require.NoError(t, err)

	for _, token := range []string{token1, token2} {
		// test
		ctx, err := srvAuth.Authenticate(t.Context(), map[string][]string{"authorization": {fmt.Sprintf("Bearer %s", token)}})

		// verify
		assert.NoError(t, err)
		assert.NotNil(t, ctx)

		// test, upper-case header
		ctx, err = srvAuth.Authenticate(t.Context(), map[string][]string{"Authorization": {fmt.Sprintf("Bearer %s", token)}})

		// verify
		assert.NoError(t, err)
		assert.NotNil(t, ctx)
	}
}

func TestOIDCAuthenticationFailedAudienceMismatchMultipleProviders(t *testing.T) {
	// prepare
	oidcServer1, err := newOIDCServer()
	require.NoError(t, err)
	oidcServer1.Start()
	defer oidcServer1.Close()

	oidcServer2, err := newOIDCServer()
	require.NoError(t, err)
	oidcServer2.Start()
	defer oidcServer2.Close()

	config := &Config{
		Providers: []ProviderCfg{
			{
				IssuerURL: oidcServer1.URL,
				Audience:  "unit-test-1",
			},
			{
				IssuerURL: oidcServer2.URL,
				Audience:  "unit-test-2",
			},
		},
	}
	p := newTestExtension(t, config)

	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	srvAuth, ok := p.(extensionauth.Server)
	require.True(t, ok)

	payload1, _ := json.Marshal(map[string]any{
		"sub":         "jdoe@example.com",
		"name":        "jdoe",
		"iss":         oidcServer1.URL,
		"aud":         "unit-test-2", // this is the mismatch
		"exp":         time.Now().Add(time.Minute).Unix(),
		"memberships": []string{"department-1", "department-2"},
	})
	token, err := oidcServer1.token(payload1)
	require.NoError(t, err)

	// test
	_, err = srvAuth.Authenticate(t.Context(), map[string][]string{"authorization": {fmt.Sprintf("Bearer %s", token)}})

	// verify
	assert.Error(t, err)
}

func TestOIDCAuthenticationFailedNoMatchingIssuers(t *testing.T) {
	oidcServer, err := newOIDCServer()
	require.NoError(t, err)
	oidcServer.Start()
	defer oidcServer.Close()

	config := &Config{
		Providers: []ProviderCfg{
			{
				IssuerURL: oidcServer.URL,
				Audience:  "unit-test-1",
			},
		},
	}
	p := newTestExtension(t, config)

	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	srvAuth, ok := p.(extensionauth.Server)
	require.True(t, ok)

	payload, _ := json.Marshal(map[string]any{
		"sub":         "jdoe@example.com",
		"name":        "jdoe",
		"iss":         "https://someotherissuer.com",
		"aud":         "unit-test-1",
		"exp":         time.Now().Add(time.Minute).Unix(),
		"memberships": []string{"department-1", "department-2"},
	})
	token, err := oidcServer.token(payload)
	require.NoError(t, err)

	// test
	ctx, err := srvAuth.Authenticate(t.Context(), map[string][]string{"authorization": {fmt.Sprintf("Bearer %s", token)}})

	// verify
	assert.Error(t, err)
	assert.NotNil(t, ctx)
}

// When only one provider is configured, we should verify against that regardless
// of the issuer in the token. In that case, the issuer provided in the extension
// config doesn't necessarily have to match the issuer in the token since it's
// only used for discovery.
// This code path doesn't actually work (for now) since Start() will fail when the issuer
// in the config doesn't match the issuer returned by the discovery server, but this test
// remains out of an abundance of caution.
func TestOIDCAuthenticationSucceededSingleIssuerMismatch(t *testing.T) {
	oidcServer, err := newOIDCServer()
	require.NoError(t, err)
	oidcServer.Start()
	defer oidcServer.Close()

	reverseProxy := newReverseProxy(t, oidcServer.URL)
	defer reverseProxy.Close()

	assert.NotEqual(t, oidcServer.URL, reverseProxy.URL)

	config := &Config{
		Providers: []ProviderCfg{
			{
				IssuerURL: reverseProxy.URL,
				Audience:  "unit-test-1",
			},
		},
	}
	p := newTestExtension(t, config)

	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.ErrorContains(t, err, "did not match")

	srvAuth, ok := p.(extensionauth.Server)
	require.True(t, ok)

	payload, _ := json.Marshal(map[string]any{
		"sub":         "jdoe@example.com",
		"name":        "jdoe",
		"iss":         oidcServer.URL,
		"aud":         "unit-test-1",
		"exp":         time.Now().Add(time.Minute).Unix(),
		"memberships": []string{"department-1", "department-2"},
	})
	token, err := oidcServer.token(payload)
	require.NoError(t, err)

	ctx, err := srvAuth.Authenticate(t.Context(), map[string][]string{"authorization": {fmt.Sprintf("Bearer %s", token)}})

	// verify
	assert.Error(t, err)
	assert.NotNil(t, ctx)
}

func TestOIDCAuthenticationSucceededIgnoreAudienceMismatch(t *testing.T) {
	// prepare
	oidcServer, err := newOIDCServer()
	require.NoError(t, err)
	oidcServer.Start()
	defer oidcServer.Close()

	config := &Config{
		IssuerURL:      oidcServer.URL,
		Audience:       "unit-test",
		IgnoreAudience: true,
	}
	p := newTestExtension(t, config)

	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	payload, _ := json.Marshal(map[string]any{
		"iss": oidcServer.URL,
		"aud": "not-unit-test",
		"exp": time.Now().Add(time.Minute).Unix(),
	})
	token, err := oidcServer.token(payload)
	require.NoError(t, err)

	srvAuth, ok := p.(extensionauth.Server)
	require.True(t, ok)

	// test
	ctx, err := srvAuth.Authenticate(t.Context(), map[string][]string{"authorization": {fmt.Sprintf("Bearer %s", token)}})

	// verify
	assert.NoError(t, err)
	assert.NotNil(t, ctx)
}

func TestOIDCAuthenticationFailAudienceMismatch(t *testing.T) {
	// prepare
	oidcServer, err := newOIDCServer()
	require.NoError(t, err)
	oidcServer.Start()
	defer oidcServer.Close()

	config := &Config{
		IssuerURL: oidcServer.URL,
		Audience:  "unit-test",
	}
	p := newTestExtension(t, config)

	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	payload, _ := json.Marshal(map[string]any{
		"iss": oidcServer.URL,
		"aud": "not-unit-test",
		"exp": time.Now().Add(time.Minute).Unix(),
	})
	token, err := oidcServer.token(payload)
	require.NoError(t, err)

	srvAuth, ok := p.(extensionauth.Server)
	require.True(t, ok)

	// test
	_, err = srvAuth.Authenticate(t.Context(), map[string][]string{"authorization": {fmt.Sprintf("Bearer %s", token)}})

	// verify
	assert.Error(t, err)
}

func TestOIDCProviderForConfigWithTLS(t *testing.T) {
	// prepare the CA cert for the TLS handler
	cert := x509.Certificate{
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(10 * time.Second),
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1)},
		SerialNumber: big.NewInt(9447457), // some number
	}
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	x509Cert, err := x509.CreateCertificate(rand.Reader, &cert, &cert, &priv.PublicKey, priv)
	require.NoError(t, err)

	caFile, err := os.CreateTemp(os.TempDir(), "cert")
	require.NoError(t, err)
	defer os.Remove(caFile.Name())

	err = pem.Encode(caFile, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: x509Cert,
	})
	require.NoError(t, err)

	oidcServer, err := newOIDCServer()
	require.NoError(t, err)
	defer oidcServer.Close()

	tlsCert := tls.Certificate{
		Certificate: [][]byte{x509Cert},
		PrivateKey:  priv,
	}
	oidcServer.TLS = &tls.Config{Certificates: []tls.Certificate{tlsCert}}
	oidcServer.StartTLS()

	// prepare the processor configuration
	config := &Config{
		IssuerURL:    oidcServer.URL,
		IssuerCAPath: caFile.Name(),
		Audience:     "unit-test",
	}

	// test
	e := &oidcExtension{
		providerContainers: make(map[string]*providerContainer),
	}
	err = e.processProviderConfig(t.Context(), config.getProviderConfigs()[0])

	// verify
	assert.NoError(t, err)
	assert.NotNil(t, e.providerContainers)
	assert.NotNil(t, e.providerContainers[oidcServer.URL].provider)
	assert.NotNil(t, e.providerContainers[oidcServer.URL].client)
	assert.NotNil(t, e.providerContainers[oidcServer.URL].transport)
}

func TestOIDCLoadIssuerCAFromPath(t *testing.T) {
	// prepare
	cert := x509.Certificate{
		SerialNumber: big.NewInt(9447457), // some number
		IsCA:         true,
	}
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	x509Cert, err := x509.CreateCertificate(rand.Reader, &cert, &cert, &priv.PublicKey, priv)
	require.NoError(t, err)

	file, err := os.CreateTemp(os.TempDir(), "cert")
	require.NoError(t, err)
	defer os.Remove(file.Name())

	err = pem.Encode(file, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: x509Cert,
	})
	require.NoError(t, err)

	// test
	loaded, err := getIssuerCACertFromPath(file.Name())

	// verify
	assert.NoError(t, err)
	assert.Equal(t, cert.SerialNumber, loaded.SerialNumber)
}

func TestOIDCFailedToLoadIssuerCAFromPathEmptyCert(t *testing.T) {
	// prepare
	file, err := os.CreateTemp(os.TempDir(), "cert")
	require.NoError(t, err)
	defer os.Remove(file.Name())

	// test
	loaded, err := getIssuerCACertFromPath(file.Name()) // the file exists, but the contents isn't a cert

	// verify
	assert.Error(t, err)
	assert.Nil(t, loaded)
}

func TestOIDCFailedToLoadIssuerCAFromPathMissingFile(t *testing.T) {
	// test
	loaded, err := getIssuerCACertFromPath("some-non-existing-file")

	// verify
	assert.Error(t, err)
	assert.Nil(t, loaded)
}

func TestOIDCFailedToLoadIssuerCAFromPathInvalidContent(t *testing.T) {
	// prepare
	file, err := os.CreateTemp(os.TempDir(), "cert")
	require.NoError(t, err)
	defer os.Remove(file.Name())
	_, err = file.WriteString("foobar")
	require.NoError(t, err)

	config := &Config{
		IssuerCAPath: file.Name(),
	}

	// test
	e := &oidcExtension{
		providerContainers: make(map[string]*providerContainer),
	}
	err = e.processProviderConfig(t.Context(), *config.getLegacyProviderConfig())

	// verify
	assert.Error(t, err)
	assert.Empty(t, e.providerContainers)
}

func TestOIDCInvalidAuthHeader(t *testing.T) {
	// prepare
	p, ok := newTestExtension(t, &Config{
		Audience:  "some-audience",
		IssuerURL: "http://example.com",
	}).(extensionauth.Server)
	require.True(t, ok)

	// test
	ctx, err := p.Authenticate(t.Context(), map[string][]string{"authorization": {"some-value"}})

	// verify
	assert.Equal(t, errInvalidAuthenticationHeaderFormat, err)
	assert.NotNil(t, ctx)
}

func TestOIDCNotAuthenticated(t *testing.T) {
	// prepare
	p, ok := newTestExtension(t, &Config{
		Audience:  "some-audience",
		IssuerURL: "http://example.com",
	}).(extensionauth.Server)
	require.True(t, ok)

	// test
	ctx, err := p.Authenticate(t.Context(), make(map[string][]string))

	// verify
	assert.Equal(t, errNotAuthenticated, err)
	assert.NotNil(t, ctx)
}

func TestProviderNotReachable(t *testing.T) {
	// prepare
	p := newTestExtension(t, &Config{
		Audience:  "some-audience",
		IssuerURL: "http://example.com",
	})

	// test
	err := p.Start(t.Context(), componenttest.NewNopHost())

	// verify
	assert.Error(t, err)

	err = p.Shutdown(t.Context())
	assert.NoError(t, err)
}

func TestFailedToVerifyToken(t *testing.T) {
	// prepare
	oidcServer, err := newOIDCServer()
	require.NoError(t, err)
	oidcServer.Start()
	defer oidcServer.Close()

	p := newTestExtension(t, &Config{
		IssuerURL: oidcServer.URL,
		Audience:  "unit-test",
	})

	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	srvAuth, ok := p.(extensionauth.Server)
	require.True(t, ok)

	// test
	ctx, err := srvAuth.Authenticate(t.Context(), map[string][]string{"authorization": {"Bearer some-token"}})

	// verify
	assert.Error(t, err)
	assert.NotNil(t, ctx)
}

func TestFailedToGetGroupsClaimFromToken(t *testing.T) {
	// prepare
	oidcServer, err := newOIDCServer()
	require.NoError(t, err)
	oidcServer.Start()
	defer oidcServer.Close()

	for _, tt := range []struct {
		casename      string
		config        *Config
		expectedError error
	}{
		{
			"groupsClaimNonExisting",
			&Config{
				IssuerURL:   oidcServer.URL,
				Audience:    "unit-test",
				GroupsClaim: "non-existing-claim",
			},
			errGroupsClaimNotFound,
		},
		{
			"usernameClaimNonExisting",
			&Config{
				IssuerURL:     oidcServer.URL,
				Audience:      "unit-test",
				UsernameClaim: "non-existing-claim",
			},
			errClaimNotFound,
		},
		{
			"usernameNotString",
			&Config{
				IssuerURL:     oidcServer.URL,
				Audience:      "unit-test",
				UsernameClaim: "some-non-string-field",
			},
			errUsernameNotString,
		},
	} {
		t.Run(tt.casename, func(t *testing.T) {
			p := newTestExtension(t, tt.config)

			err = p.Start(t.Context(), componenttest.NewNopHost())
			require.NoError(t, err)

			payload, _ := json.Marshal(map[string]any{
				"iss":                   oidcServer.URL,
				"some-non-string-field": 123,
				"aud":                   "unit-test",
				"exp":                   time.Now().Add(time.Minute).Unix(),
			})
			token, err := oidcServer.token(payload)
			require.NoError(t, err)

			srvAuth, ok := p.(extensionauth.Server)
			require.True(t, ok)

			// test
			ctx, err := srvAuth.Authenticate(t.Context(), map[string][]string{"authorization": {fmt.Sprintf("Bearer %s", token)}})

			// verify
			assert.ErrorIs(t, err, tt.expectedError)
			assert.NotNil(t, ctx)
		})
	}
}

func TestSubjectFromClaims(t *testing.T) {
	// prepare
	claims := map[string]any{
		"username": "jdoe",
	}

	// test
	username, err := getSubjectFromClaims(claims, "username", "")

	// verify
	assert.NoError(t, err)
	assert.Equal(t, "jdoe", username)
}

func TestSubjectFallback(t *testing.T) {
	// prepare
	claims := map[string]any{
		"sub": "jdoe",
	}

	// test
	username, err := getSubjectFromClaims(claims, "", "jdoe")

	// verify
	assert.NoError(t, err)
	assert.Equal(t, "jdoe", username)
}

func TestGroupsFromClaim(t *testing.T) {
	// prepare
	for _, tt := range []struct {
		casename string
		input    any
		expected []string
	}{
		{
			"single-string",
			"department-1",
			[]string{"department-1"},
		},
		{
			"multiple-strings",
			[]string{"department-1", "department-2"},
			[]string{"department-1", "department-2"},
		},
		{
			"multiple-things",
			[]any{"department-1", 123},
			[]string{"department-1", "123"},
		},
	} {
		t.Run(tt.casename, func(t *testing.T) {
			claims := map[string]any{
				"sub":         "jdoe",
				"memberships": tt.input,
			}

			// test
			groups, err := getGroupsFromClaims(claims, "memberships")
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, groups)
		})
	}
}

func TestEmptyGroupsClaim(t *testing.T) {
	// prepare
	claims := map[string]any{
		"sub": "jdoe",
	}

	// test
	groups, err := getGroupsFromClaims(claims, "")
	assert.NoError(t, err)
	assert.Equal(t, []string{}, groups)
}

func TestMissingClient(t *testing.T) {
	// prepare
	config := &Config{
		IssuerURL: "http://example.com/",
	}

	// test
	err := config.Validate()

	// verify
	assert.Equal(t, errNoAudienceProvided, err)
}

func TestIgnoreMissingClient(t *testing.T) {
	// prepare
	config := &Config{
		IssuerURL:      "http://example.com/",
		IgnoreAudience: true,
	}

	// test
	err := config.Validate()

	// verify
	assert.NoError(t, err)
}

func TestMissingIssuerURL(t *testing.T) {
	// prepare
	config := &Config{
		Audience: "some-audience",
	}

	// test
	err := config.Validate()

	// verify
	assert.Equal(t, errNoIssuerURL, err)
}

func TestShutdown(t *testing.T) {
	// prepare
	config := &Config{
		Audience:  "some-audience",
		IssuerURL: "http://example.com/",
	}
	p := newTestExtension(t, config)
	require.NotNil(t, p)

	// test
	err := p.Shutdown(t.Context()) // for now, we never fail

	// verify
	assert.NoError(t, err)
}
