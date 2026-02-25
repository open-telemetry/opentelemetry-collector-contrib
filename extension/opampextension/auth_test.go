// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opampextension

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"
	"google.golang.org/grpc/credentials"
)

func TestMakeHeadersFunc(t *testing.T) {
	t.Run("Nil server config", func(t *testing.T) {
		headersFunc, err := makeHeadersFunc(zap.NewNop(), nil, nil)
		require.NoError(t, err)
		require.Nil(t, headersFunc)
	})

	t.Run("No auth extension specified", func(t *testing.T) {
		headersFunc, err := makeHeadersFunc(zap.NewNop(), &OpAMPServer{
			WS: &commonFields{},
		}, nil)
		require.NoError(t, err)
		require.Nil(t, headersFunc)
	})

	t.Run("Extension does not exist", func(t *testing.T) {
		nopHost := componenttest.NewNopHost()
		headersFunc, err := makeHeadersFunc(zap.NewNop(), &OpAMPServer{
			WS: &commonFields{
				Auth: component.NewID(component.MustNewType("bearerauth")),
			},
		}, nopHost)
		require.EqualError(t, err, `could not find auth extension "bearerauth"`)
		require.Nil(t, headersFunc)
	})

	t.Run("Extension is not an auth extension", func(t *testing.T) {
		authComponent := component.NewID(component.MustNewType("bearerauth"))
		host := &mockHost{
			extensions: map[component.ID]component.Component{
				authComponent: mockComponent{},
			},
		}
		headersFunc, err := makeHeadersFunc(zap.NewNop(), &OpAMPServer{
			WS: &commonFields{
				Auth: authComponent,
			},
		}, host)

		require.EqualError(t, err, `auth extension "bearerauth" is not an extensionauth.HTTPClient`)
		require.Nil(t, headersFunc)
	})

	t.Run("Headers func extracts headers from extension", func(t *testing.T) {
		authComponent := component.NewID(component.MustNewType("bearerauth"))
		h := http.Header{}
		h.Set("Authorization", "Bearer user:pass")

		host := &mockHost{
			extensions: map[component.ID]component.Component{
				authComponent: mockAuthClient{
					header: h,
				},
			},
		}
		headersFunc, err := makeHeadersFunc(zap.NewNop(), &OpAMPServer{
			WS: &commonFields{
				Auth: authComponent,
			},
		}, host)

		require.NoError(t, err)
		headersOut := headersFunc(http.Header{
			"OtherHeader": []string{"OtherValue"},
		})

		require.Equal(t, http.Header{
			"OtherHeader":   []string{"OtherValue"},
			"Authorization": []string{"Bearer user:pass"},
		}, headersOut)
	})
}

type mockHost struct {
	extensions map[component.ID]component.Component
}

func (m mockHost) GetExtensions() map[component.ID]component.Component {
	return m.extensions
}

type mockComponent struct{}

func (mockComponent) Start(_ context.Context, _ component.Host) error { return nil }
func (mockComponent) Shutdown(_ context.Context) error                { return nil }

type mockAuthClient struct {
	header http.Header
}

func (mockAuthClient) Start(_ context.Context, _ component.Host) error { return nil }
func (mockAuthClient) Shutdown(_ context.Context) error                { return nil }
func (m mockAuthClient) RoundTripper(base http.RoundTripper) (http.RoundTripper, error) {
	return mockRoundTripper{
		header: m.header,
		base:   base,
	}, nil
}

func (mockAuthClient) PerRPCCredentials() (credentials.PerRPCCredentials, error) {
	return nil, nil
}

type mockRoundTripper struct {
	header http.Header
	base   http.RoundTripper
}

func (m mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	reqClone := req.Clone(req.Context())

	for k, vals := range m.header {
		for _, val := range vals {
			reqClone.Header.Add(k, val)
		}
	}

	return m.base.RoundTrip(reqClone)
}
