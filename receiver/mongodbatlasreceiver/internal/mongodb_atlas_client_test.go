// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.uber.org/zap"
)

func TestNewMongoDBAtlasClientUserAgent(t *testing.T) {
	var gotUserAgent string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserAgent = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[],"links":[]}`))
	}))
	defer srv.Close()

	client, err := NewMongoDBAtlasClient(
		srv.URL,
		"public",
		"private",
		configretry.NewDefaultBackOffConfig(),
		zap.NewNop(),
		component.BuildInfo{Command: "otelcol-contrib", Version: "1.2.3"},
	)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, client.Shutdown()) })

	_, err = client.Organizations(t.Context())
	require.NoError(t, err)

	assert.Regexp(t, `^otelcol-contrib/1\.2\.3 go-mongodbatlas/\S+ \(\w+;\w+\)$`, gotUserAgent)
}
