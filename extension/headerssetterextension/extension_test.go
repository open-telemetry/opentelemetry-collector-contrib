// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package headerssetterextension

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/config/configopaque"
)

type mockRoundTripper struct{}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
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
				context.Background(),
				client.Info{
					Metadata: tt.metadata,
				},
			)
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, "", nil)
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
				context.Background(),
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
