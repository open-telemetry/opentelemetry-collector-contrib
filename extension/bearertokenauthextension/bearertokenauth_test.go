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

package bearertokenauthextension

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
)

func TestPerRPCAuth(t *testing.T) {
	metadata := map[string]string{
		"authorization": "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
	}

	// test meta data is properly
	perRPCAuth := &PerRPCAuth{metadata: metadata}
	md, err := perRPCAuth.GetRequestMetadata(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, md, metadata)

	// always true
	ok := perRPCAuth.RequireTransportSecurity()
	assert.True(t, ok)
}

type mockRoundTripper struct{}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp := &http.Response{StatusCode: http.StatusOK, Header: map[string][]string{}}
	for k, v := range req.Header {
		resp.Header.Set(k, v[0])
	}
	return resp, nil
}

func TestBearerAuthenticatorHttp(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.BearerToken = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."

	bauth := newBearerTokenAuth(cfg, nil)
	assert.NotNil(t, bauth)

	base := &mockRoundTripper{}
	c, err := bauth.RoundTripper(base)
	assert.NoError(t, err)
	assert.NotNil(t, c)

	request := &http.Request{Method: "Get"}
	resp, err := c.RoundTrip(request)
	assert.NoError(t, err)
	authHeaderValue := resp.Header.Get("Authorization")
	assert.Equal(t, authHeaderValue, fmt.Sprintf("Bearer %s", cfg.BearerToken))
}

func TestBearerAuthenticator(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.BearerToken = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."

	bauth := newBearerTokenAuth(cfg, nil)
	assert.NotNil(t, bauth)

	assert.Nil(t, bauth.Start(context.Background(), componenttest.NewNopHost()))
	credential, err := bauth.PerRPCCredentials()

	assert.NoError(t, err)
	assert.NotNil(t, credential)

	md, err := credential.GetRequestMetadata(context.Background())
	expectedMd := map[string]string{
		"authorization": fmt.Sprintf("Bearer %s", cfg.BearerToken),
	}
	assert.Equal(t, md, expectedMd)
	assert.NoError(t, err)
	assert.True(t, credential.RequireTransportSecurity())

	roundTripper, _ := bauth.RoundTripper(&mockRoundTripper{})
	orgHeaders := http.Header{
		"Foo": {"bar"},
	}
	expectedHeaders := http.Header{
		"Foo":           {"bar"},
		"Authorization": {bauth.bearerToken()},
	}

	resp, err := roundTripper.RoundTrip(&http.Request{Header: orgHeaders})
	assert.NoError(t, err)
	assert.Equal(t, expectedHeaders, resp.Header)
	assert.Nil(t, bauth.Shutdown(context.Background()))
}
