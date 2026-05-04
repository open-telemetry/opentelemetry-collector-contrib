// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package extensions

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"
)

type mockAuthExtension struct {
	extension.Extension
	rt     http.RoundTripper
	rtErr  error
	header http.Header
}

func (m *mockAuthExtension) RoundTripper(base http.RoundTripper) (http.RoundTripper, error) {
	if m.rtErr != nil {
		return nil, m.rtErr
	}
	if m.rt != nil {
		return m.rt, nil
	}
	return &mockRoundTripper{base: base, add: m.header}, nil
}

// mockRoundTripper sets the configured headers on the request then delegates
// to the base round tripper (which in production is the capturing one).
type mockRoundTripper struct {
	base http.RoundTripper
	add  http.Header
	err  error
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	for k, vs := range m.add {
		for _, v := range vs {
			req.Header.Set(k, v)
		}
	}
	return m.base.RoundTrip(req)
}

// nonAuthExtension is a plain extension without the extensionauth.HTTPClient
// interface, to exercise the type-assert failure path.
type nonAuthExtension struct {
	extension.Extension
}

func TestMakeHeadersFunc(t *testing.T) {
	authID := component.MustNewID("bearerauth")

	tests := []struct {
		name    string
		exts    map[component.ID]component.Component
		wantErr string
		// check runs after MakeHeadersFunc returns the closure; inspect
		// the produced headers for the happy paths.
		check func(t *testing.T, fn func(http.Header) http.Header)
	}{
		{
			name: "happy path adds auth header",
			exts: map[component.ID]component.Component{
				authID: &mockAuthExtension{
					header: http.Header{"Authorization": []string{"Bearer test-token"}},
				},
			},
			check: func(t *testing.T, fn func(http.Header) http.Header) {
				out := fn(http.Header{})
				require.Equal(t, "Bearer test-token", out.Get("Authorization"))
			},
		},
		{
			name: "preserves existing static headers",
			exts: map[component.ID]component.Component{
				authID: &mockAuthExtension{
					header: http.Header{"Authorization": []string{"Bearer test-token"}},
				},
			},
			check: func(t *testing.T, fn func(http.Header) http.Header) {
				out := fn(http.Header{"X-Custom": []string{"foo"}})
				require.Equal(t, "foo", out.Get("X-Custom"))
				require.Equal(t, "Bearer test-token", out.Get("Authorization"))
			},
		},
		{
			name:    "nil exts map",
			exts:    nil,
			wantErr: `auth extension "bearerauth" not found`,
		},
		{
			name: "extension not found",
			exts: map[component.ID]component.Component{
				component.MustNewID("other"): &mockAuthExtension{},
			},
			wantErr: `auth extension "bearerauth" not found`,
		},
		{
			name: "extension is not HTTPClient",
			exts: map[component.ID]component.Component{
				authID: &nonAuthExtension{},
			},
			wantErr: `does not implement extensionauth.HTTPClient`,
		},
		{
			name: "RoundTripper setup error",
			exts: map[component.ID]component.Component{
				authID: &mockAuthExtension{rtErr: errors.New("boom")},
			},
			wantErr: "failed to create auth round tripper",
		},
		{
			name: "round-trip error at call time returns original headers",
			exts: map[component.ID]component.Component{
				authID: &mockAuthExtension{
					rt: &mockRoundTripper{err: errors.New("refresh failed")},
				},
			},
			check: func(t *testing.T, fn func(http.Header) http.Header) {
				in := http.Header{"X-Custom": []string{"foo"}}
				out := fn(in)
				// Input headers are returned unchanged; no auth header is set.
				require.Equal(t, "foo", out.Get("X-Custom"))
				require.Empty(t, out.Get("Authorization"))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fn, err := MakeHeadersFunc(zap.NewNop(), authID, tc.exts)
			if tc.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantErr)
				require.Nil(t, fn)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, fn)
			if tc.check != nil {
				tc.check(t, fn)
			}
		})
	}
}
