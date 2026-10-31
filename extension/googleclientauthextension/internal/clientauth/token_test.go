// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clientauth // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/googleclientauthextension/internal/clientauth"

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/extension"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/grpc/credentials/oauth"
)

const testPrincipal = "kafka-client@my-project.iam.gserviceaccount.com"

type errorTokenSource struct{}

func (errorTokenSource) Token() (*oauth2.Token, error) {
	return nil, errors.New("token error")
}

// decodeGoogleManagedKafkaToken splits a google_managed_kafka token into its decoded header,
// claims and access token.
func decodeGoogleManagedKafkaToken(t *testing.T, token string) (header, claims, accessToken string) {
	t.Helper()
	parts := strings.Split(token, ".")
	require.Len(t, parts, 3)
	decoded := make([]string, len(parts))
	for i, part := range parts {
		b, err := base64.RawURLEncoding.DecodeString(part)
		require.NoError(t, err)
		decoded[i] = string(b)
	}
	return decoded[0], decoded[1], decoded[2]
}

func newStartedAuthenticator(tokenType string, source oauth2.TokenSource) *clientAuthenticator {
	return &clientAuthenticator{
		TokenSource: &oauth.TokenSource{TokenSource: source},
		config:      &Config{TokenType: tokenType},
		principal:   testPrincipal,
	}
}

func TestWrapGoogleManagedKafkaToken(t *testing.T) {
	expiry := time.Unix(1700003600, 0)
	now := time.Unix(1700000000, 0)

	token, err := wrapGoogleManagedKafkaToken(&oauth2.Token{AccessToken: "ya29.access-token", Expiry: expiry}, testPrincipal, now)
	require.NoError(t, err)

	header, claims, accessToken := decodeGoogleManagedKafkaToken(t, token.AccessToken)
	assert.JSONEq(t, `{"typ":"JWT","alg":"GOOG_OAUTH2_TOKEN"}`, header)
	assert.JSONEq(t, `{"exp":1700003600,"iat":1700000000,"iss":"Google","sub":"kafka-client@my-project.iam.gserviceaccount.com"}`, claims)
	assert.Equal(t, "ya29.access-token", accessToken)
	assert.Equal(t, "Bearer", token.TokenType)
	assert.Equal(t, expiry, token.Expiry)
}

func TestToken_GoogleManagedKafka(t *testing.T) {
	expiry := time.Now().Add(time.Hour)
	ca := newStartedAuthenticator(googleManagedKafka, oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "ya29.access-token", Expiry: expiry}))

	token, err := ca.Token(t.Context())
	require.NoError(t, err)

	_, claims, accessToken := decodeGoogleManagedKafkaToken(t, token.AccessToken)
	var c googleManagedKafkaClaims
	require.NoError(t, json.Unmarshal([]byte(claims), &c))
	assert.Equal(t, testPrincipal, c.Subject)
	assert.Equal(t, "Google", c.Issuer)
	assert.Equal(t, expiry.Unix(), c.ExpiresAt)
	assert.Equal(t, "ya29.access-token", accessToken)
}

func TestToken_AccessTokenPassThrough(t *testing.T) {
	ca := newStartedAuthenticator(accessToken, oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "ya29.access-token"}))

	token, err := ca.Token(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "ya29.access-token", token.AccessToken)
}

func TestToken_NotStarted(t *testing.T) {
	ca := &clientAuthenticator{config: &Config{TokenType: googleManagedKafka}}

	_, err := ca.Token(t.Context())
	assert.ErrorContains(t, err, "not started")
}

func TestToken_CanceledContext(t *testing.T) {
	ca := newStartedAuthenticator(googleManagedKafka, oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "ya29.access-token"}))
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := ca.Token(ctx)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestToken_SourceError(t *testing.T) {
	ca := newStartedAuthenticator(googleManagedKafka, errorTokenSource{})

	_, err := ca.Token(t.Context())
	assert.ErrorContains(t, err, "token error")
}

func TestResolvePrincipal(t *testing.T) {
	tests := []struct {
		name            string
		principal       string
		credsJSON       string
		getDefaultEmail func(context.Context) (string, error)
		want            string
		wantErr         string
	}{
		{
			name:      "config takes precedence",
			principal: "configured@my-project.iam.gserviceaccount.com",
			credsJSON: `{"type":"service_account","client_email":"key@my-project.iam.gserviceaccount.com"}`,
			want:      "configured@my-project.iam.gserviceaccount.com",
		},
		{
			name:      "service account key",
			credsJSON: `{"type":"service_account","client_email":"key@my-project.iam.gserviceaccount.com"}`,
			want:      "key@my-project.iam.gserviceaccount.com",
		},
		{
			name:      "credentials without client_email",
			credsJSON: `{"type":"authorized_user","client_id":"id","client_secret":"secret","refresh_token":"token"}`,
			wantErr:   "no principal found in application default credentials",
		},
		{
			name: "metadata server",
			getDefaultEmail: func(context.Context) (string, error) {
				return "gsa@my-project.iam.gserviceaccount.com", nil
			},
			want: "gsa@my-project.iam.gserviceaccount.com",
		},
		{
			name: "metadata server error",
			getDefaultEmail: func(context.Context) (string, error) {
				return "", errors.New("metadata unavailable")
			},
			wantErr: "metadata unavailable",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ca := &clientAuthenticator{
				config:          &Config{TokenType: googleManagedKafka, Principal: tt.principal},
				getDefaultEmail: tt.getDefaultEmail,
			}
			got, err := ca.resolvePrincipal(t.Context(), &google.Credentials{JSON: []byte(tt.credsJSON)})
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestStart_GoogleManagedKafkaWithoutPrincipal(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "testdata/fake_creds.json")
	cfg := CreateDefaultConfig().(*Config)
	cfg.TokenType = googleManagedKafka
	ext, err := CreateExtension(t.Context(), extension.Settings{}, cfg)
	require.NoError(t, err)

	err = ext.Start(t.Context(), nil)
	assert.ErrorContains(t, err, "set principal in the config")
}

func TestStart_GoogleManagedKafkaWithPrincipal(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "testdata/fake_creds.json")
	cfg := CreateDefaultConfig().(*Config)
	cfg.TokenType = googleManagedKafka
	cfg.Principal = testPrincipal
	ext, err := CreateExtension(t.Context(), extension.Settings{}, cfg)
	require.NoError(t, err)

	require.NoError(t, ext.Start(t.Context(), nil))
	assert.Equal(t, testPrincipal, ext.(*clientAuthenticator).principal)
}

func TestToken_GoogleManagedKafkaFromMetadataServer(t *testing.T) {
	// Mimic the metadata server, as seen from a GKE pod using Workload Identity.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/computeMetadata/v1/instance/service-accounts/default/email":
			_, _ = w.Write([]byte("gsa@my-project.iam.gserviceaccount.com"))
		case "/computeMetadata/v1/instance/service-accounts/default/token":
			_ = json.NewEncoder(w).Encode(fakeMetadataToken{AccessToken: "ya29.metadata-token", TokenType: "Bearer", ExpiresIn: 3600})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	t.Setenv("GCE_METADATA_HOST", srv.Listener.Addr().String())
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "")
	t.Setenv("HOME", t.TempDir())

	cfg := CreateDefaultConfig().(*Config)
	cfg.Project = "my-project"
	cfg.TokenType = googleManagedKafka
	ext, err := CreateExtension(t.Context(), extension.Settings{}, cfg)
	require.NoError(t, err)
	require.NoError(t, ext.Start(t.Context(), nil))

	token, err := ext.(contextTokenSource).Token(t.Context())
	require.NoError(t, err)

	header, claims, accessToken := decodeGoogleManagedKafkaToken(t, token.AccessToken)
	if accessToken != "ya29.metadata-token" {
		// Don't print this out in-case it is a real access token.
		t.Fatal("access token was incorrect. FindDefaultCredentials may have found real credentials.")
	}
	assert.JSONEq(t, googleManagedKafkaHeader, header)
	var c googleManagedKafkaClaims
	require.NoError(t, json.Unmarshal([]byte(claims), &c))
	assert.Equal(t, "gsa@my-project.iam.gserviceaccount.com", c.Subject)
	assert.Equal(t, token.Expiry.Unix(), c.ExpiresAt)
}
