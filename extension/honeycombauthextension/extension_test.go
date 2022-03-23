package honeycombauthextension

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"net/http"
	"testing"
)

type mockRoundTripper struct{}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp := &http.Response{StatusCode: http.StatusOK, Header: map[string][]string{}}
	for k, v := range req.Header {
		resp.Header.Set(k, v[0])
	}
	return resp, nil
}

func TestPerRPCAuth(t *testing.T) {
	metadata := map[string]string{
		teamMetadataKey:    "test-team",
		datasetMetadataKey: "test-dataset",
	}

	perRPCAuth := &PerRPCCredentials{metadata: metadata}
	md, err := perRPCAuth.GetRequestMetadata(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, md, metadata)

	ok := perRPCAuth.RequireTransportSecurity()
	assert.True(t, ok)
}

func TestClientAuthenticator(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Team = "test-team"
	cfg.Dataset = "test-dataset"

	auth := newClientAuthenticator(cfg)

	err := auth.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	credential, err := auth.PerRPCCredentials()

	assert.NoError(t, err)
	assert.NotNil(t, credential)

	md, err := credential.GetRequestMetadata(context.Background())
	expectedMd := map[string]string{
		teamMetadataKey:    "test-team",
		datasetMetadataKey: "test-dataset",
	}
	assert.Equal(t, md, expectedMd)
	assert.NoError(t, err)
	assert.True(t, credential.RequireTransportSecurity())

	roundTripper, _ := auth.RoundTripper(&mockRoundTripper{})
	orgHeaders := http.Header{
		teamMetadataKey:    {"test-team"},
		datasetMetadataKey: {"test-dataset"},
	}
	expectedHeaders := http.Header{
		http.CanonicalHeaderKey(teamMetadataKey):    {"test-team"},
		http.CanonicalHeaderKey(datasetMetadataKey): {"test-dataset"},
	}

	resp, err := roundTripper.RoundTrip(&http.Request{Header: orgHeaders})
	assert.NoError(t, err)
	assert.Equal(t, expectedHeaders, resp.Header)

	err = auth.Shutdown(context.Background())
	assert.NoError(t, err)
}
