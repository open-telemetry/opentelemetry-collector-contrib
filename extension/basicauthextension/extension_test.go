// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package basicauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/basicauthextension"

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap/zaptest"
)

var credentials = [][]string{
	{"htpasswd-md5", "$apr1$FVVioVP7$ZdIWPG1p4E/ErujO7kA2n0"},
	{"openssl-apr1", "$apr1$peiE49Vv$lo.z77Z.6.a.Lm7GMjzQh0"},
	{"openssl-md5", "$1$mvmz31IB$U9KpHBLegga2doA0e3s3N0"},
	{"htpasswd-sha", "{SHA}vFznddje0Ht4+pmO0FaxwrUKN/M="},
	{"htpasswd-bcrypt", "$2y$10$Q6GeMFPd0dAxhQULPDdAn.DFy6NDmLaU0A7e2XoJz7PFYAEADFKbC"},
	{"", "$2a$06$DCq7YPn5Rq63x1Lad4cll.TV4S6ytwfsfvkgY8jIucDrjc8deX1s."},
	{"", "$2a$08$HqWuK6/Ng6sg9gQzbLrgb.Tl.ZHfXLhvt/SgVyWhQqgqcZ7ZuUtye"},
	{"", "$2a$10$k1wbIrmNyFAPwPVPSVa/zecw2BCEnBwVS2GbrmgzxFUOqW9dk4TCW"},
	{"", "$2a$12$k42ZFHFWqBp3vWli.nIn8uYyIkbvYRvodzbfbK18SSsY.CsIQPlxO"},
	{"a", "$2a$06$m0CrhHm10qJ3lXRY.5zDGO3rS2KdeeWLuGmsfGlMfOxih58VYVfxe"},
	{"a", "$2a$08$cfcvVd2aQ8CMvoMpP2EBfeodLEkkFJ9umNEfPD18.hUF62qqlC/V."},
	{"a", "$2a$10$k87L/MF28Q673VKh8/cPi.SUl7MU/rWuSiIDDFayrKk/1tBsSQu4u"},
	{"a", "$2a$12$8NJH3LsPrANStV6XtBakCez0cKHXVxmvxIlcz785vxAIZrihHZpeS"},
	{"abc", "$2a$06$If6bvum7DFjUnE9p2uDeDu0YHzrHM6tf.iqN8.yx.jNN1ILEf7h0i"},
	{"abcdefghijklmnopqrstuvwxyz", "$2a$06$.rCVZVOThsIa97pEDOxvGuRRgzG64bvtJ0938xuqzv18d3ZpQhstC"},
	{"abcdefghijklmnopqrstuvwxyz", "$2a$08$aTsUwsyowQuzRrDqFflhgekJ8d9/7Z3GV3UcgvzQW3J5zMyrTvlz."},
	{"abcdefghijklmnopqrstuvwxyz", "$2a$10$fVH8e28OQRj9tqiDXs1e1uxpsjN0c7II7YPKXua2NAKYvM6iQk7dq"},
	{"abcdefghijklmnopqrstuvwxyz", "$2a$12$D4G5f18o7aMMfwasBL7GpuQWuP3pkrZrOAnqP.bmezbMng.QwJ/pG"},
	{"~!@#$%^&*()      ~!@#$%^&*()PNBFRD", "$2a$06$fPIsBO8qRqkjj273rfaOI.HtSV9jLDpTbZn782DC6/t7qT67P6FfO"},
	{"~!@#$%^&*()      ~!@#$%^&*()PNBFRD", "$2a$08$Eq2r4G/76Wv39MzSX262huzPz612MZiYHVUJe/OcOql2jo4.9UxTW"},
	{"~!@#$%^&*()      ~!@#$%^&*()PNBFRD", "$2a$10$LgfYWkbzEvQ4JakH7rOvHe0y8pHKF9OaFgwUZ2q7W2FFZmZzJYlfS"},
	{"~!@#$%^&*()      ~!@#$%^&*()PNBFRD", "$2a$12$WApznUOJfkEGSmYRfnkrPOr466oFDCaj4b6HY3EXGvfxm43seyhgC"},
	{"ππππππππ", "$2a$10$.TtQJ4Jr6isd4Hp.mVfZeuh6Gws4rOQ/vdBczhDx.19NFK0Y84Dle"},
}

func TestBasicAuth_Valid(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	for _, c := range credentials {
		_, err := fmt.Fprintf(&buf, "%s:%s\n", c[0], c[1])
		require.NoError(t, err)
	}
	htpasswdFile := filepath.Join(t.TempDir(), ".htpasswd")
	err := os.WriteFile(htpasswdFile, buf.Bytes(), 0o600)
	require.NoError(t, err)

	ctx := t.Context()

	ext, err := newServerAuthExtension(&Config{
		Htpasswd: &HtpasswdSettings{
			File: htpasswdFile,
		},
	})
	require.NoError(t, err)

	require.NoError(t, ext.Start(ctx, componenttest.NewNopHost()))

	for _, c := range credentials {
		t.Run(c[0], func(t *testing.T) {
			t.Parallel()
			auth := fmt.Sprintf("%s:%s", c[0], c[0])
			auth = base64.StdEncoding.EncodeToString([]byte(auth))

			authCtx, err := ext.Authenticate(ctx, map[string][]string{"authorization": {"Basic " + auth}})
			assert.NoError(t, err)
			cl := client.FromContext(authCtx)
			assert.Equal(t, c[0], cl.Auth.GetAttribute("username"))
			assert.Equal(t, auth, cl.Auth.GetAttribute("raw"))
		})
	}
}

func TestBasicAuth_InvalidCredentials(t *testing.T) {
	ext, err := newServerAuthExtension(&Config{
		Htpasswd: &HtpasswdSettings{
			Inline: "username:password",
		},
	})
	require.NoError(t, err)
	require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))
	_, err = ext.Authenticate(t.Context(), map[string][]string{"authorization": {"Basic dXNlcm5hbWU6cGFzc3dvcmR4eHg="}})
	assert.Equal(t, errInvalidCredentials, err)
}

func TestBasicAuth_NoHeader(t *testing.T) {
	ext, err := newServerAuthExtension(&Config{
		Htpasswd: &HtpasswdSettings{
			Inline: "username:password",
		},
	})
	require.NoError(t, err)
	require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))
	_, err = ext.Authenticate(t.Context(), map[string][]string{})
	assert.Equal(t, errNoAuth, err)
}

func TestBasicAuth_InvalidPrefix(t *testing.T) {
	ext, err := newServerAuthExtension(&Config{
		Htpasswd: &HtpasswdSettings{
			Inline: "username:password",
		},
	})
	require.NoError(t, err)
	require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))
	_, err = ext.Authenticate(t.Context(), map[string][]string{"authorization": {"Bearer token"}})
	assert.Equal(t, errInvalidSchemePrefix, err)
}

func TestBasicAuth_NoFile(t *testing.T) {
	ext, err := newServerAuthExtension(&Config{
		Htpasswd: &HtpasswdSettings{
			File: "/non/existing/file",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, ext)

	require.Error(t, ext.Start(t.Context(), componenttest.NewNopHost()))
}

func TestBasicAuth_InvalidFormat(t *testing.T) {
	ext, err := newServerAuthExtension(&Config{
		Htpasswd: &HtpasswdSettings{
			Inline: "username:password",
		},
	})
	require.NoError(t, err)
	require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))
	for _, auth := range [][]string{
		{"non decodable", "invalid"},
		{"missing separator", "aW52YWxpZAo="},
	} {
		t.Run(auth[0], func(t *testing.T) {
			_, err = ext.Authenticate(t.Context(), map[string][]string{"authorization": {"Basic " + auth[1]}})
			assert.Equal(t, errInvalidFormat, err)
		})
	}
}

func TestBasicAuth_HtpasswdInlinePrecedence(t *testing.T) {
	t.Parallel()

	htpasswdFile := filepath.Join(t.TempDir(), ".htpasswd")
	err := os.WriteFile(htpasswdFile, []byte("username:fromfile"), 0o600)
	require.NoError(t, err)

	ext, err := newServerAuthExtension(&Config{
		Htpasswd: &HtpasswdSettings{
			File:   htpasswdFile,
			Inline: "username:frominline",
		},
	})
	require.NoError(t, err)
	require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))

	auth := base64.StdEncoding.EncodeToString([]byte("username:frominline"))

	_, err = ext.Authenticate(t.Context(), map[string][]string{"authorization": {"Basic " + auth}})
	assert.NoError(t, err)

	auth = base64.StdEncoding.EncodeToString([]byte("username:fromfile"))

	_, err = ext.Authenticate(t.Context(), map[string][]string{"authorization": {"Basic " + auth}})
	assert.ErrorIs(t, errInvalidCredentials, err)
}

func TestBasicAuth_SupportedHeaders(t *testing.T) {
	ext, err := newServerAuthExtension(&Config{
		Htpasswd: &HtpasswdSettings{
			Inline: "username:password",
		},
	})
	require.NoError(t, err)
	require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))

	auth := base64.StdEncoding.EncodeToString([]byte("username:password"))

	for _, k := range []string{
		"Authorization",
		"authorization",
		"aUtHoRiZaTiOn",
	} {
		_, err = ext.Authenticate(t.Context(), map[string][]string{k: {"Basic " + auth}})
		assert.NoError(t, err)
	}
}

func TestBasicAuth_ServerInvalid(t *testing.T) {
	_, err := newServerAuthExtension(&Config{
		Htpasswd: &HtpasswdSettings{},
	})
	assert.Error(t, err)
}

func TestPerRPCAuth(t *testing.T) {
	t.Parallel()
	ext := &basicAuthClient{
		clientAuth: &ClientAuthSettings{
			Username: "username",
			Password: "passwordxxx",
		},
	}
	require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))

	rpcAuth, err := ext.PerRPCCredentials()
	require.NoError(t, err)
	md, err := rpcAuth.GetRequestMetadata(t.Context())
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{
		"authorization": "Basic dXNlcm5hbWU6cGFzc3dvcmR4eHg=",
	}, md)

	ok := rpcAuth.RequireTransportSecurity()
	assert.True(t, ok)
}

type mockRoundTripper struct{}

func (*mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp := &http.Response{StatusCode: http.StatusOK, Header: map[string][]string{}}
	maps.Copy(resp.Header, req.Header)
	return resp, nil
}

func TestBasicAuth_ClientValid(t *testing.T) {
	ext := newClientAuthExtension(&Config{
		ClientAuth: &ClientAuthSettings{
			Username: "username",
			Password: "password",
		},
	})
	require.NotNil(t, ext)

	require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))

	base := &mockRoundTripper{}
	c, err := ext.RoundTripper(base)
	require.NoError(t, err)
	require.NotNil(t, c)

	authCreds := base64.StdEncoding.EncodeToString([]byte("username:password"))
	orgHeaders := http.Header{
		"Test-Header-1": []string{"test-value-1"},
	}
	expectedHeaders := http.Header{
		"Test-Header-1": []string{"test-value-1"},
		"Authorization": {fmt.Sprintf("Basic %s", authCreds)},
	}

	resp, err := c.RoundTrip(&http.Request{Header: orgHeaders})
	assert.NoError(t, err)
	assert.Equal(t, expectedHeaders, resp.Header)

	credential, err := ext.PerRPCCredentials()

	assert.NoError(t, err)
	assert.NotNil(t, credential)

	md, err := credential.GetRequestMetadata(t.Context())
	expectedMd := map[string]string{
		"authorization": fmt.Sprintf("Basic %s", authCreds),
	}
	assert.Equal(t, expectedMd, md)
	assert.NoError(t, err)
	assert.True(t, credential.RequireTransportSecurity())

	assert.NoError(t, ext.Shutdown(t.Context()))
}

func TestBasicAuth_ClientInvalid(t *testing.T) {
	t.Run("invalid username format", func(t *testing.T) {
		ext := newClientAuthExtension(&Config{
			ClientAuth: &ClientAuthSettings{
				Username: "user:name",
				Password: "password",
			},
		})
		require.NotNil(t, ext)

		require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))

		base := &mockRoundTripper{}
		_, err := ext.RoundTripper(base)
		assert.Error(t, err)

		_, err = ext.PerRPCCredentials()
		assert.Error(t, err)
	})
}

type mockSecretProvider struct {
	secret   atomic.Pointer[string]
	onChange func(string)
}

func (m *mockSecretProvider) Start(context.Context, component.Host) error { return nil }
func (m *mockSecretProvider) Shutdown(context.Context) error              { return nil }

func (m *mockSecretProvider) GetSecret(_ context.Context) (string, error) {
	s := m.secret.Load()
	if s == nil {
		return "", fmt.Errorf("no secret configured")
	}
	return *s, nil
}

func (m *mockSecretProvider) OnChange(fn func(string)) {
	m.onChange = fn
}

func (m *mockSecretProvider) setSecret(s string) {
	m.secret.Store(&s)
}

func (m *mockSecretProvider) simulateRotation(s string) {
	m.secret.Store(&s)
	if m.onChange != nil {
		m.onChange(s)
	}
}

// mockHost wraps componenttest.NewNopHost() and adds custom extensions.
type mockHost struct {
	component.Host
	extensions map[component.ID]component.Component
}

func (h *mockHost) GetExtensions() map[component.ID]component.Component {
	return h.extensions
}

func newMockHost(exts map[component.ID]component.Component) *mockHost {
	return &mockHost{
		Host:       componenttest.NewNopHost(),
		extensions: exts,
	}
}

func TestClientAuth_SecretProvider(t *testing.T) {
	t.Parallel()
	secret := map[string]string{"user": "admin", "pass": "secret123"}
	data, _ := json.Marshal(secret)

	mock := &mockSecretProvider{}
	mock.setSecret(string(data))

	providerID := component.MustNewID("testprovider")
	host := newMockHost(map[component.ID]component.Component{
		providerID: mock,
	})

	ext := &basicAuthClient{
		clientAuth: &ClientAuthSettings{
			SecretProvider: &SecretProviderConfig{
				ID:          providerID,
				UsernameKey: "user",
				PasswordKey: "pass",
			},
		},
		logger: zaptest.NewLogger(t),
	}

	require.NoError(t, ext.Start(t.Context(), host))
	defer func() { require.NoError(t, ext.Shutdown(t.Context())) }()

	assert.Equal(t, "admin", ext.Username())
	assert.Equal(t, "secret123", ext.Password())

	rt, err := ext.RoundTripper(&mockRoundTripper{})
	require.NoError(t, err)

	resp, err := rt.RoundTrip(&http.Request{Method: http.MethodGet})
	require.NoError(t, err)
	assert.Contains(t, resp.Header.Get("Authorization"), "Basic ")
}

func TestServerAuth_SecretProvider(t *testing.T) {
	t.Parallel()

	mock := &mockSecretProvider{}
	mock.setSecret("testuser:{SHA}W6ph5Mm5Pz8GgiULbPgzG37mj9g=")

	providerID := component.MustNewID("testprovider")
	host := newMockHost(map[component.ID]component.Component{
		providerID: mock,
	})

	ext := &basicAuthServer{
		htpasswd: &HtpasswdSettings{
			SecretProvider: &SecretProviderConfig{
				ID: providerID,
			},
		},
		logger: zaptest.NewLogger(t),
	}

	require.NoError(t, ext.Start(t.Context(), host))
	defer func() { require.NoError(t, ext.Shutdown(t.Context())) }()

	auth := "dGVzdHVzZXI6cGFzc3dvcmQ=" // testuser:password
	ctx, err := ext.Authenticate(t.Context(), map[string][]string{"authorization": {"Basic " + auth}})
	assert.NoError(t, err)
	assert.NotNil(t, ctx)
}

func TestSecretProvider_ExtensionNotFound(t *testing.T) {
	t.Parallel()
	host := newMockHost(map[component.ID]component.Component{})

	ext := &basicAuthServer{
		htpasswd: &HtpasswdSettings{
			SecretProvider: &SecretProviderConfig{
				ID: component.MustNewID("nonexistent"),
			},
		},
		logger: zaptest.NewLogger(t),
	}

	err := ext.Start(t.Context(), host)
	assert.ErrorContains(t, err, "not found")
}

func TestSecretProvider_ExtensionNotImplemented(t *testing.T) {
	t.Parallel()
	providerID := component.MustNewID("notaprovider")
	host := newMockHost(map[component.ID]component.Component{
		providerID: nopExtension{},
	})

	ext := &basicAuthServer{
		htpasswd: &HtpasswdSettings{
			SecretProvider: &SecretProviderConfig{
				ID: providerID,
			},
		},
		logger: zaptest.NewLogger(t),
	}

	err := ext.Start(t.Context(), host)
	assert.ErrorContains(t, err, "does not implement SecretProvider")
}

func TestDependencies_Server(t *testing.T) {
	t.Parallel()
	providerID := component.MustNewID("myprovider")

	ext := &basicAuthServer{
		htpasswd: &HtpasswdSettings{
			SecretProvider: &SecretProviderConfig{ID: providerID},
		},
	}
	assert.Equal(t, []component.ID{providerID}, ext.Dependencies())

	extNoProvider := &basicAuthServer{
		htpasswd: &HtpasswdSettings{Inline: "user:pass"},
	}
	assert.Nil(t, extNoProvider.Dependencies())
}

func TestDependencies_Client(t *testing.T) {
	t.Parallel()
	providerID := component.MustNewID("myprovider")

	ext := &basicAuthClient{
		clientAuth: &ClientAuthSettings{
			SecretProvider: &SecretProviderConfig{ID: providerID, UsernameKey: "u", PasswordKey: "p"},
		},
	}
	assert.Equal(t, []component.ID{providerID}, ext.Dependencies())

	extNoProvider := &basicAuthClient{
		clientAuth: &ClientAuthSettings{Username: "user", Password: "pass"},
	}
	assert.Nil(t, extNoProvider.Dependencies())
}

type nopExtension struct{}

func (nopExtension) Start(context.Context, component.Host) error { return nil }
func (nopExtension) Shutdown(context.Context) error              { return nil }
