// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuredataexplorerexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuredataexplorerexporter"

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

type scopedCredential struct {
	ctx     context.Context
	options []policy.TokenRequestOptions
}

func (c *scopedCredential) GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	c.ctx = ctx
	c.options = append(c.options, opts)
	return azcore.AccessToken{Token: "test-token", ExpiresOn: time.Now().Add(time.Hour)}, nil
}

func TestIngestV2AuthenticationScopes(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		require.Equal(t, "/v1/rest/auth/metadata", r.URL.Path)
		require.Empty(t, r.Header.Get("Authorization"))
		_, _ = io.WriteString(w, `{"AzureAD":{"LoginEndpoint":"https://login.microsoftonline.com",
			"LoginMfaRequired":true,"KustoServiceResourceId":"https://kusto.kusto.windows.net"}}`)
	}))
	defer server.Close()
	cfg := Config{ApplicationID: "00000000-0000-0000-0000-000000000001",
		TenantID: "00000000-0000-0000-0000-000000000002", ApplicationKey: "test-secret"}
	auth := &ingestV2Authentication{config: cfg, dmURL: server.URL, httpClient: server.Client(), logger: zaptest.NewLogger(t)}
	_, scope, err := auth.initialize(t.Context())
	require.NoError(t, err)
	require.Equal(t, "https://kusto.kusto.windows.net/.default", scope)
	credential := &scopedCredential{}
	auth.credential = credential
	token, err := auth.token(t.Context())
	require.NoError(t, err)
	require.Equal(t, "test-token", token)
	storageOpts := policy.TokenRequestOptions{Scopes: []string{"https://storage.azure.com/.default"}, Claims: "storage-claims"}
	_, err = auth.GetToken(t.Context(), storageOpts)
	require.NoError(t, err)
	require.Equal(t, []string{scope}, credential.options[0].Scopes)
	require.Equal(t, storageOpts, credential.options[1])
	require.Equal(t, t.Context(), credential.ctx)
	require.Equal(t, 1, requests)
}

func TestIngestV2CredentialsPreserveAuthModes(t *testing.T) {
	for _, tc := range []struct {
		name     string
		cfg      Config
		expected azcore.TokenCredential
	}{
		{"application", Config{ApplicationID: "00000000-0000-0000-0000-000000000001", TenantID: "00000000-0000-0000-0000-000000000002", ApplicationKey: "test-secret"}, &azidentity.ClientSecretCredential{}},
		{"default", Config{UseAzureAuth: true}, &azidentity.DefaultAzureCredential{}},
		{"system", Config{ManagedIdentityID: " SYSTEM "}, &azidentity.ManagedIdentityCredential{}},
		{"user", Config{ManagedIdentityID: "00000000-0000-0000-0000-000000000001"}, &azidentity.ManagedIdentityCredential{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			credential, err := newIngestV2Credential(&tc.cfg, publicLoginEndpoint, &http.Client{})
			require.NoError(t, err)
			require.IsType(t, tc.expected, credential)
		})
	}
}

func TestIngestV2MetadataFailuresAndCancellation(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  int
		body    string
		wantErr bool
	}{
		{"not-found", http.StatusNotFound, "not found", false},
		{"service-error", http.StatusServiceUnavailable, "unavailable", false},
		{"empty", http.StatusOK, "", false},
		{"no-aad", http.StatusOK, `{}`, false},
		{"malformed", http.StatusOK, `{invalid`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()
			info, err := ingestV2CloudInfo(t.Context(), server.URL, server.Client(), zaptest.NewLogger(t))
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, publicKustoAudience, info.KustoServiceResourceID)
			}
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			_, err = ingestV2CloudInfo(ctx, server.URL, server.Client(), zaptest.NewLogger(t))
			require.ErrorIs(t, err, context.Canceled)
		})
	}
}

func TestIngestV2MetadataNetworkFailureDoesNotUseDefaults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	server.Close()
	_, err := ingestV2CloudInfo(t.Context(), server.URL, server.Client(), zaptest.NewLogger(t))
	require.Error(t, err)
}
