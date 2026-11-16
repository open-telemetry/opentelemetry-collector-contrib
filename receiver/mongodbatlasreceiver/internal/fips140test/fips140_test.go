// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:debug fips140=only

package fips140test

import (
	"crypto/fips140"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver/internal"
)

// The server answers like Atlas does, with an MD5 digest challenge.
func TestDigestAuthUnderFIPS140Only(t *testing.T) {
	require.True(t, fips140.Enabled(), "//go:debug fips140=only not applied")

	var authorized bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.Header().Set("WWW-Authenticate", `Digest realm="MMS Public API", domain="", nonce="abc123", algorithm=MD5, qop="auth", stale=false`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		authorized = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[],"links":[]}`))
	}))
	defer srv.Close()

	client, err := internal.NewMongoDBAtlasClient(srv.URL, "public", "private", configretry.NewDefaultBackOffConfig(), zap.NewNop(), component.BuildInfo{})
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, client.Shutdown()) })

	_, err = client.Organizations(t.Context())
	require.NoError(t, err)
	assert.True(t, authorized)
}
