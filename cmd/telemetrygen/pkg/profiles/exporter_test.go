// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package profiles

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/pprofile/pprofileotlp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/internal/config"
)

type captureServer struct {
	pprofileotlp.UnimplementedGRPCServer
	mu       sync.Mutex
	metadata metadata.MD
}

func (s *captureServer) Export(ctx context.Context, _ pprofileotlp.ExportRequest) (pprofileotlp.ExportResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	md, _ := metadata.FromIncomingContext(ctx)
	s.metadata = md
	return pprofileotlp.NewExportResponse(), nil
}

func TestGRPCExporterForwardsHeaders(t *testing.T) {
	capture := &captureServer{}
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	srv := grpc.NewServer()
	pprofileotlp.RegisterGRPCServer(srv, capture)
	go func() { _ = srv.Serve(lis) }()
	defer srv.Stop()

	cfg := &Config{
		Config: config.Config{
			CustomEndpoint: lis.Addr().String(),
			Insecure:       true,
			Headers:        config.KeyValue{"authorization": "Bearer test-token", "x-custom": "value"},
		},
	}

	exp, err := newGRPCExporter(cfg)
	require.NoError(t, err)
	defer func() { _ = exp.Shutdown(t.Context()) }()

	td := pprofile.NewProfiles()
	td.ResourceProfiles().AppendEmpty()
	require.NoError(t, exp.Export(t.Context(), td))

	capture.mu.Lock()
	md := capture.metadata
	capture.mu.Unlock()

	require.NotNil(t, md, "server should have received metadata")
	assert.Equal(t, []string{"Bearer test-token"}, md.Get("authorization"))
	assert.Equal(t, []string{"value"}, md.Get("x-custom"))
}

func TestHTTPExporterUsesDefaultProfilesPath(t *testing.T) {
	var requestPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	cfg := newTestHTTPExporterConfig(t, srv.URL)

	exp, err := newHTTPExporter(cfg)
	require.NoError(t, err)
	defer func() { _ = exp.Shutdown(t.Context()) }()

	expectedEndpoint := "http://" + cfg.CustomEndpoint + "/v1development/profiles"
	assert.Equal(t, expectedEndpoint, exp.endpoint)

	td := pprofile.NewProfiles()
	td.ResourceProfiles().AppendEmpty()
	require.NoError(t, exp.Export(t.Context(), td))
	assert.Equal(t, "/v1development/profiles", requestPath)
}

func TestHTTPExporterForwardsHeaders(t *testing.T) {
	var (
		authorization string
		customHeader  string
		contentType   string
		bodySize      int
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization = r.Header.Get("authorization")
		customHeader = r.Header.Get("x-custom")
		contentType = r.Header.Get("Content-Type")
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		bodySize = len(body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	cfg := newTestHTTPExporterConfig(t, srv.URL)
	cfg.Headers = config.KeyValue{"authorization": "Bearer test-token", "x-custom": "value"}

	exp, err := newHTTPExporter(cfg)
	require.NoError(t, err)
	defer func() { _ = exp.Shutdown(t.Context()) }()

	td := pprofile.NewProfiles()
	td.ResourceProfiles().AppendEmpty()
	require.NoError(t, exp.Export(t.Context(), td))

	assert.Equal(t, "Bearer test-token", authorization)
	assert.Equal(t, "value", customHeader)
	assert.Equal(t, "application/x-protobuf", contentType)
	assert.Positive(t, bodySize)
}

func TestHTTPExporterReturnsErrorOnNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusBadGateway)
	}))
	defer srv.Close()

	cfg := newTestHTTPExporterConfig(t, srv.URL)

	exp, err := newHTTPExporter(cfg)
	require.NoError(t, err)
	defer func() { _ = exp.Shutdown(t.Context()) }()

	td := pprofile.NewProfiles()
	td.ResourceProfiles().AppendEmpty()
	err = exp.Export(t.Context(), td)
	require.Error(t, err)
	assert.EqualError(t, err, "HTTP export failed with status 502")
}

func newTestHTTPExporterConfig(t *testing.T, rawURL string) *Config {
	t.Helper()

	parsedURL, err := url.Parse(rawURL)
	require.NoError(t, err)

	cfg := NewConfig()
	cfg.Insecure = true
	cfg.CustomEndpoint = parsedURL.Host
	return cfg
}
