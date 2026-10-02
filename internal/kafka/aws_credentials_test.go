// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafka

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoadAWSCredentialsProvider_ExpiryWindow(t *testing.T) {
	for _, tt := range []struct {
		name      string
		lifetime  time.Duration
		wantCalls int32
		wantKey   string
	}{
		{name: "reuse valid credentials", lifetime: 5 * time.Minute, wantCalls: 1, wantKey: "AKID1"},
		{name: "refresh near-expiry credentials", lifetime: 30 * time.Second, wantCalls: 2, wantKey: "AKID2"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// Given a real web identity provider backed by a local STS endpoint.
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := r.ParseForm(); err != nil {
					t.Error(err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				if r.Form.Get("Action") != "AssumeRoleWithWebIdentity" {
					t.Errorf("unexpected STS action: %s", r.Form.Get("Action"))
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				call := calls.Add(1)
				lifetime := tt.lifetime
				if call > 1 {
					lifetime = time.Hour
				}
				w.Header().Set("Content-Type", "text/xml")
				_, err := fmt.Fprintf(w, `<AssumeRoleWithWebIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/"><AssumeRoleWithWebIdentityResult><Credentials><AccessKeyId>AKID%d</AccessKeyId><SecretAccessKey>SECRET</SecretAccessKey><SessionToken>TOKEN</SessionToken><Expiration>%s</Expiration></Credentials></AssumeRoleWithWebIdentityResult></AssumeRoleWithWebIdentityResponse>`, call, time.Now().Add(lifetime).UTC().Format(time.RFC3339))
				if err != nil {
					t.Error(err)
				}
			}))
			t.Cleanup(server.Close)

			dir := t.TempDir()
			tokenFile := filepath.Join(dir, "token")
			require.NoError(t, os.WriteFile(tokenFile, []byte("test-web-identity-token"), 0o600))
			for _, name := range []string{"AWS_ACCESS_KEY_ID", "AWS_ACCESS_KEY", "AWS_SECRET_ACCESS_KEY", "AWS_SECRET_KEY", "AWS_SESSION_TOKEN", "AWS_PROFILE", "AWS_DEFAULT_PROFILE"} {
				t.Setenv(name, "")
			}
			t.Setenv("AWS_CONFIG_FILE", filepath.Join(dir, "config"))
			t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(dir, "credentials"))
			t.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", tokenFile)
			t.Setenv("AWS_ROLE_ARN", "arn:aws:iam::123456789012:role/test")
			t.Setenv("AWS_ROLE_SESSION_NAME", "test-session")
			t.Setenv("AWS_ENDPOINT_URL_STS", server.URL)
			t.Setenv("AWS_IGNORE_CONFIGURED_ENDPOINT_URLS", "false")
			provider, err := loadAWSCredentialsProvider(t.Context(), "us-west-2")
			require.NoError(t, err)
			_, err = provider.Retrieve(t.Context())
			require.NoError(t, err)

			// When credentials are requested again without waiting for expiration.
			creds, err := provider.Retrieve(t.Context())

			// Then only credentials inside the expiry window are refreshed.
			require.NoError(t, err)
			require.Equal(t, tt.wantKey, creds.AccessKeyID)
			require.Equal(t, tt.wantCalls, calls.Load())
		})
	}
}
