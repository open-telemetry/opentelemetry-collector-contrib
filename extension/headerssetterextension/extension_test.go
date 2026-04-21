// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package headerssetterextension

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configopaque"
)

type mockRoundTripper struct{}

func (*mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp := &http.Response{StatusCode: http.StatusOK, Header: map[string][]string{}}
	for k, v := range req.Header {
		resp.Header.Set(k, v[0])
	}
	return resp, nil
}

func TestRoundTripper(t *testing.T) {
	for _, tt := range tests {
		t.Run("round_tripper", func(t *testing.T) {
			ext, err := newHeadersSetterExtension(tt.cfg, nil)
			assert.NoError(t, err)
			assert.NotNil(t, ext)

			roundTripper, err := ext.RoundTripper(mrt)
			assert.NoError(t, err)
			assert.NotNil(t, roundTripper)

			ctx := client.NewContext(
				t.Context(),
				client.Info{
					Metadata: tt.metadata,
				},
			)
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, "", http.NoBody)
			assert.NoError(t, err)
			assert.NotNil(t, req)

			resp, err := roundTripper.RoundTrip(req)
			assert.NoError(t, err)
			assert.NotNil(t, resp)
			for _, header := range tt.cfg.HeadersConfig {
				assert.Equal(
					t,
					resp.Header.Get(*header.Key),
					tt.expectedHeaders[*header.Key],
				)
			}
		})
	}
}

func TestPerRPCCredentials(t *testing.T) {
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			ext, err := newHeadersSetterExtension(tt.cfg, nil)
			assert.NoError(t, err)
			assert.NotNil(t, ext)

			perRPC, err := ext.PerRPCCredentials()
			assert.NoError(t, err)
			assert.NotNil(t, perRPC)

			ctx := client.NewContext(
				t.Context(),
				client.Info{Metadata: tt.metadata},
			)

			metadata, err := perRPC.GetRequestMetadata(ctx)
			assert.NoError(t, err)
			assert.NotNil(t, metadata)
			for _, header := range tt.cfg.HeadersConfig {
				assert.Equal(
					t,
					metadata[*header.Key],
					tt.expectedHeaders[*header.Key],
				)
			}
		})
	}
}

var (
	mrt           = &mockRoundTripper{}
	headername    = "header_name"
	anotherHeader = "another_header_name"
	tests         = []struct {
		cfg             *Config
		metadata        client.Metadata
		expectedHeaders map[string]string
	}{
		{
			cfg: &Config{
				HeadersConfig: []HeaderConfig{
					{
						Key:         &headername,
						Action:      INSERT,
						FromContext: stringp("tenant"),
					},
				},
			},
			metadata: client.NewMetadata(
				map[string][]string{"tenant": {"context value"}},
			),
			expectedHeaders: map[string]string{
				"header_name": "context value",
			},
		},
		{
			cfg: &Config{
				HeadersConfig: []HeaderConfig{
					{
						Key:    &headername,
						Action: INSERT,
						Value:  stringp("config value"),
					},
				},
			},
			expectedHeaders: map[string]string{
				"header_name": "config value",
			},
		},
		{
			cfg: &Config{
				HeadersConfig: []HeaderConfig{
					{
						Key:         &headername,
						Action:      INSERT,
						FromContext: stringp("tenant"),
					},
					{
						Key:         &anotherHeader,
						Action:      INSERT,
						FromContext: stringp("tenant"),
					},
				},
			},
			metadata: client.NewMetadata(
				map[string][]string{"tenant": {"acme"}},
			),
			expectedHeaders: map[string]string{
				"header_name":         "acme",
				"another_header_name": "acme",
			},
		},
		{
			cfg: &Config{
				HeadersConfig: []HeaderConfig{
					{
						Key:         &headername,
						Action:      INSERT,
						FromContext: stringp(""),
					},
				},
			},
			expectedHeaders: map[string]string{
				"header_name": "",
			},
		},
		{
			cfg: &Config{
				HeadersConfig: []HeaderConfig{
					{
						Key:    &headername,
						Action: INSERT,
						Value:  stringp(""),
					},
				},
			},
			expectedHeaders: map[string]string{
				"header_name": "",
			},
		},
		{
			cfg: &Config{
				HeadersConfig: []HeaderConfig{
					{
						Key:         &headername,
						Action:      INSERT,
						FromContext: stringp("tenant"),
					},
					{
						Key:         &anotherHeader,
						Action:      INSERT,
						FromContext: stringp("tenant_"),
					},
				},
			},
			metadata: client.NewMetadata(
				map[string][]string{"tenant": {"acme"}},
			),
			expectedHeaders: map[string]string{
				"header_name":         "acme",
				"another_header_name": "",
			},
		},
		{
			cfg: &Config{
				HeadersConfig: []HeaderConfig{
					{
						Key:         &headername,
						Action:      INSERT,
						FromContext: stringp("tenant_"),
					},
				},
			},
			metadata: client.NewMetadata(
				map[string][]string{"tenant": {"acme"}},
			),
			expectedHeaders: map[string]string{
				"header_name": "",
			},
		},
		{
			cfg: &Config{
				HeadersConfig: []HeaderConfig{
					{
						Key:          &headername,
						Action:       INSERT,
						FromContext:  stringp("tenant"),
						DefaultValue: opaquep("default_tenant"),
					},
				},
			},
			metadata: client.NewMetadata(
				map[string][]string{},
			),
			expectedHeaders: map[string]string{
				"header_name": "default_tenant",
			},
		},
		{
			cfg: &Config{
				HeadersConfig: []HeaderConfig{
					{
						Key:          &headername,
						Action:       INSERT,
						FromContext:  stringp("tenant"),
						DefaultValue: opaquep("default_tenant"),
					},
				},
			},
			metadata: client.NewMetadata(
				map[string][]string{"tenant": {"acme"}},
			),
			expectedHeaders: map[string]string{
				"header_name": "acme",
			},
		},
	}
)

func stringp(str string) *string {
	return &str
}

func opaquep(stro configopaque.String) *configopaque.String {
	return &stro
}

func TestRoundTripper_FileSource(t *testing.T) {
	dir := t.TempDir()
	apiKeyFile := filepath.Join(dir, "api-key")
	require.NoError(t, os.WriteFile(apiKeyFile, []byte("secret123"), 0o600))

	headerKey := "X-API-Key"
	cfg := &Config{
		HeadersConfig: []HeaderConfig{
			{
				Key:       &headerKey,
				Action:    UPSERT,
				ValueFile: &apiKeyFile,
			},
		},
	}

	ext, err := newHeadersSetterExtension(cfg, componenttest.NewNopTelemetrySettings().Logger)
	require.NoError(t, err)
	require.NotNil(t, ext)

	require.NoError(t, ext.start(t.Context(), componenttest.NewNopHost()))
	defer func() { require.NoError(t, ext.shutdown(t.Context())) }()

	roundTripper, err := ext.RoundTripper(mrt)
	require.NoError(t, err)
	require.NotNil(t, roundTripper)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "", http.NoBody)
	require.NoError(t, err)

	resp, err := roundTripper.RoundTrip(req)
	require.NoError(t, err)
	assert.Equal(t, "secret123", resp.Header.Get("X-API-Key"))
}

func TestRoundTripper_FileSource_UpdatesOnChange(t *testing.T) {
	dir := t.TempDir()
	apiKeyFile := filepath.Join(dir, "api-key")
	require.NoError(t, os.WriteFile(apiKeyFile, []byte("original"), 0o600))

	headerKey := "X-API-Key"
	cfg := &Config{
		HeadersConfig: []HeaderConfig{
			{
				Key:       &headerKey,
				Action:    UPSERT,
				ValueFile: &apiKeyFile,
			},
		},
	}

	ext, err := newHeadersSetterExtension(cfg, componenttest.NewNopTelemetrySettings().Logger)
	require.NoError(t, err)
	require.NoError(t, ext.start(t.Context(), componenttest.NewNopHost()))
	defer func() { require.NoError(t, ext.shutdown(t.Context())) }()

	roundTripper, err := ext.RoundTripper(mrt)
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "", http.NoBody)
	require.NoError(t, err)

	resp, err := roundTripper.RoundTrip(req)
	require.NoError(t, err)
	assert.Equal(t, "original", resp.Header.Get("X-API-Key"))

	// Update the file
	require.NoError(t, os.WriteFile(apiKeyFile, []byte("updated"), 0o600))

	// Verify the header value updates
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "", http.NoBody)
		assert.NoError(c, err)
		resp, err := roundTripper.RoundTrip(req)
		assert.NoError(c, err)
		assert.Equal(c, "updated", resp.Header.Get("X-API-Key"))
	}, 5*time.Second, 50*time.Millisecond)
}

func TestPerRPCCredentials_FileSource(t *testing.T) {
	dir := t.TempDir()
	tokenFile := filepath.Join(dir, "token")
	require.NoError(t, os.WriteFile(tokenFile, []byte("bearer-token-123"), 0o600))

	headerKey := "Authorization"
	cfg := &Config{
		HeadersConfig: []HeaderConfig{
			{
				Key:       &headerKey,
				Action:    INSERT,
				ValueFile: &tokenFile,
			},
		},
	}

	ext, err := newHeadersSetterExtension(cfg, componenttest.NewNopTelemetrySettings().Logger)
	require.NoError(t, err)
	require.NoError(t, ext.start(t.Context(), componenttest.NewNopHost()))
	defer func() { require.NoError(t, ext.shutdown(t.Context())) }()

	perRPC, err := ext.PerRPCCredentials()
	require.NoError(t, err)
	require.NotNil(t, perRPC)

	metadata, err := perRPC.GetRequestMetadata(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "bearer-token-123", metadata["Authorization"])
}

func TestPerRPCCredentials_FileSource_UpdatesOnChange(t *testing.T) {
	dir := t.TempDir()
	tokenFile := filepath.Join(dir, "token")
	require.NoError(t, os.WriteFile(tokenFile, []byte("token-v1"), 0o600))

	headerKey := "X-Auth-Token"
	cfg := &Config{
		HeadersConfig: []HeaderConfig{
			{
				Key:       &headerKey,
				Action:    UPSERT,
				ValueFile: &tokenFile,
			},
		},
	}

	ext, err := newHeadersSetterExtension(cfg, componenttest.NewNopTelemetrySettings().Logger)
	require.NoError(t, err)
	require.NoError(t, ext.start(t.Context(), componenttest.NewNopHost()))
	defer func() { require.NoError(t, ext.shutdown(t.Context())) }()

	perRPC, err := ext.PerRPCCredentials()
	require.NoError(t, err)

	metadata, err := perRPC.GetRequestMetadata(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "token-v1", metadata["X-Auth-Token"])

	// Update the file
	require.NoError(t, os.WriteFile(tokenFile, []byte("token-v2"), 0o600))

	// Verify the metadata updates
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		metadata, err := perRPC.GetRequestMetadata(t.Context())
		assert.NoError(c, err)
		assert.Equal(c, "token-v2", metadata["X-Auth-Token"])
	}, 5*time.Second, 50*time.Millisecond)
}

func TestFileSource_MultipleHeaders(t *testing.T) {
	dir := t.TempDir()
	apiKeyFile := filepath.Join(dir, "api-key")
	tokenFile := filepath.Join(dir, "token")
	require.NoError(t, os.WriteFile(apiKeyFile, []byte("key123"), 0o600))
	require.NoError(t, os.WriteFile(tokenFile, []byte("token456"), 0o600))

	apiKeyHeader := "X-API-Key" //nolint:gosec // G101 - header name, not a credential
	tokenHeader := "X-Token"
	cfg := &Config{
		HeadersConfig: []HeaderConfig{
			{
				Key:       &apiKeyHeader,
				Action:    UPSERT,
				ValueFile: &apiKeyFile,
			},
			{
				Key:       &tokenHeader,
				Action:    INSERT,
				ValueFile: &tokenFile,
			},
		},
	}

	ext, err := newHeadersSetterExtension(cfg, componenttest.NewNopTelemetrySettings().Logger)
	require.NoError(t, err)
	require.NoError(t, ext.start(t.Context(), componenttest.NewNopHost()))
	defer func() { require.NoError(t, ext.shutdown(t.Context())) }()

	roundTripper, err := ext.RoundTripper(mrt)
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "", http.NoBody)
	require.NoError(t, err)

	resp, err := roundTripper.RoundTrip(req)
	require.NoError(t, err)
	assert.Equal(t, "key123", resp.Header.Get("X-API-Key"))
	assert.Equal(t, "token456", resp.Header.Get("X-Token"))
}
