// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// nolint:errcheck
package lumigoauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/basicauthextension"

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"
)

type mockRoundTripper struct{}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp := &http.Response{StatusCode: http.StatusOK, Header: map[string][]string{}}
	for k, v := range req.Header {
		resp.Header[k] = v
	}
	return resp, nil
}

func TestLumigoAuth_Client_NoTokenAnywhere(t *testing.T) {
	ext, err := newClientAuthExtension(&Config{
		Type: Client,
	}, zap.NewNop())
	require.NoError(t, err)

	base := &mockRoundTripper{}
	c, err := ext.RoundTripper(base)
	require.NoError(t, err)
	require.NotNil(t, c)

	orgHeaders := http.Header{
		"Test-Header-1": []string{"test-value-1"},
	}

	_, err = c.RoundTrip(&http.Request{Header: orgHeaders})
	assert.ErrorContains(t, err, "no Lumigo token set in the configurations, and none found in the context")
}

func TestLumigoAuth_Client_TokenInConfig(t *testing.T) {
	testToken := "t_123456789012345678901"
	ext, err := newClientAuthExtension(&Config{
		Type:  Client,
		Token: testToken,
	}, zap.NewNop())
	require.NoError(t, err)

	base := &mockRoundTripper{}
	c, err := ext.RoundTripper(base)
	require.NoError(t, err)
	require.NotNil(t, c)

	orgHeaders := http.Header{
		"Test-Header-1": []string{"test-value-1"},
	}
	expectedHeaders := http.Header{
		"Test-Header-1": []string{"test-value-1"},
		"Authorization": []string{"LumigoToken " + testToken},
	}

	resp, err := c.RoundTrip(&http.Request{Header: orgHeaders})
	assert.NoError(t, err)
	assert.Equal(t, expectedHeaders, resp.Header)
}

func TestLumigoAuth_Client_TokenInContext(t *testing.T) {
	testToken := "t_123456789012345678901"
	ext, err := newClientAuthExtension(&Config{
		Type: Client,
	}, zap.NewNop())
	require.NotNil(t, ext)
	require.NoError(t, err)

	require.NoError(t, ext.Start(context.Background(), componenttest.NewNopHost()))

	base := &mockRoundTripper{}
	c, err := ext.RoundTripper(base)
	require.NoError(t, err)
	require.NotNil(t, c)

	orgHeaders := http.Header{
		"Test-Header-1": []string{"test-value-1"},
	}
	expectedHeaders := http.Header{
		"Test-Header-1": []string{"test-value-1"},
		"Authorization": []string{"LumigoToken " + testToken},
	}

	req := &http.Request{Header: orgHeaders}
	req = req.WithContext(context.WithValue(req.Context(), lumigoTokenContextKey, testToken))
	resp, err := c.RoundTrip(req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, expectedHeaders, resp.Header)
}

func TestLumigoAuth_Server_InvalidPrefix(t *testing.T) {
	ext, err := newServerAuthExtension(&Config{}, zap.NewNop())
	require.NoError(t, err)
	_, err = ext.Authenticate(context.Background(), map[string][]string{"authorization": {"Bearer token"}})
	assert.Equal(t, errInvalidSchemePrefix, err)
}

func TestLumigoAuth_Server_SupportedHeaders(t *testing.T) {
	ext, err := newServerAuthExtension(&Config{}, zap.NewNop())
	require.NoError(t, err)
	require.NoError(t, ext.Start(context.Background(), componenttest.NewNopHost()))

	for _, k := range []string{
		"Authorization",
		"authorization",
		"aUtHoRiZaTiOn",
	} {
		_, err = ext.Authenticate(context.Background(), map[string][]string{k: {"LumigoToken 1234567"}})
		assert.NoError(t, err)
	}
}
