// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlesecretmanagerprovider

import (
	"context"
	"fmt"
	"testing"

	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	gax "github.com/googleapis/gax-go/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc/codes"
)

// mockSecretsManagerClient is a fake Google Secret Manager client for unit tests.
type mockSecretsManagerClient struct {
	validSecrets map[string]string
	clientClosed bool
}

func (m *mockSecretsManagerClient) AccessSecretVersion(_ context.Context, req *secretmanagerpb.AccessSecretVersionRequest, _ ...gax.CallOption) (*secretmanagerpb.AccessSecretVersionResponse, error) {
	secretString, ok := m.validSecrets[req.Name]
	if !ok {
		return nil, fmt.Errorf("secrets entry does not exist, error code: %v", codes.NotFound)
	}
	return &secretmanagerpb.AccessSecretVersionResponse{
		Payload: &secretmanagerpb.SecretPayload{
			Data: []byte(secretString),
		},
	}, nil
}

func (m *mockSecretsManagerClient) Close() error {
	m.clientClosed = true
	return nil
}

// newTestProvider creates a provider with a pre-injected mock client.
func newTestProvider(t *testing.T, secrets map[string]string) *provider {
	t.Helper()
	return &provider{
		client: &mockSecretsManagerClient{validSecrets: secrets},
		logger: zaptest.NewLogger(t),
	}
}

// ── Happy-path tests ──────────────────────────────────────────────────────────

func TestProvider_Retrieve_PlainSecret(t *testing.T) {
	t.Parallel()
	p := newTestProvider(t, map[string]string{
		"projects/my-project/secrets/secret-1/versions/1": "plain-value",
	})

	got, err := p.Retrieve(t.Context(), schemeName+":projects/my-project/secrets/secret-1/versions/1", nil)
	require.NoError(t, err)

	val, err := got.AsString()
	require.NoError(t, err)
	assert.Equal(t, "plain-value", val)
}

func TestProvider_Retrieve_JSONKeyExtraction(t *testing.T) {
	t.Parallel()
	p := newTestProvider(t, map[string]string{
		"projects/my-project/secrets/db-creds/versions/latest": `{"username":"admin","password":"s3cr3t"}`,
	})

	got, err := p.Retrieve(t.Context(),
		schemeName+":projects/my-project/secrets/db-creds/versions/latest#password", nil)
	require.NoError(t, err)

	val, err := got.AsString()
	require.NoError(t, err)
	assert.Equal(t, "s3cr3t", val)
}

func TestProvider_Retrieve_DefaultValueWhenSecretNameEmpty(t *testing.T) {
	t.Parallel()
	// URI: googlesecretmanager::-fallback-val
	// After stripping scheme prefix → spec = ":-fallback-val"
	// strings.Cut(spec, ":-") → selector="", defaultValue="fallback-val", hasDefaultValue=true
	// The provider short-circuits and returns the default without calling the API.
	p := newTestProvider(t, nil)

	got, err := p.Retrieve(t.Context(), schemeName+"::-fallback-val", nil)
	require.NoError(t, err)

	val, err := got.AsString()
	require.NoError(t, err)
	assert.Equal(t, "fallback-val", val)
}

func TestProvider_Retrieve_DefaultValueWhenJSONKeyAbsent(t *testing.T) {
	t.Parallel()
	p := newTestProvider(t, map[string]string{
		"projects/my-project/secrets/db-creds/versions/1": `{"username":"admin"}`,
	})

	// #password key is missing in the JSON → should use the default
	got, err := p.Retrieve(t.Context(),
		schemeName+":projects/my-project/secrets/db-creds/versions/1#password:-default-pwd", nil)
	require.NoError(t, err)

	val, err := got.AsString()
	require.NoError(t, err)
	assert.Equal(t, "default-pwd", val)
}

func TestProvider_Retrieve_JSONKeyAndDefaultCombined(t *testing.T) {
	t.Parallel()
	p := newTestProvider(t, map[string]string{
		"projects/my-project/secrets/db-creds/versions/2": `{"username":"admin","password":"s3cr3t"}`,
	})

	// Key exists → default is ignored
	got, err := p.Retrieve(t.Context(),
		schemeName+":projects/my-project/secrets/db-creds/versions/2#username:-nobody", nil)
	require.NoError(t, err)

	val, err := got.AsString()
	require.NoError(t, err)
	assert.Equal(t, "admin", val)
}

// ── Failure tests ─────────────────────────────────────────────────────────────

func TestProvider_Retrieve_InvalidScheme(t *testing.T) {
	t.Parallel()
	p := newTestProvider(t, nil)

	_, err := p.Retrieve(t.Context(), "invalidscheme:projects/my-project/secrets/test/versions/1", nil)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrURINotSupported)
}

func TestProvider_Retrieve_SecretNotFound(t *testing.T) {
	t.Parallel()
	p := newTestProvider(t, map[string]string{
		"projects/my-project/secrets/secret-1/versions/1": "value",
	})

	_, err := p.Retrieve(t.Context(),
		schemeName+":projects/my-project/secrets/non-existent/versions/1", nil)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrAccessSecretVersion)
}

func TestProvider_Retrieve_JSONKeyNotFoundWithoutDefault(t *testing.T) {
	t.Parallel()
	p := newTestProvider(t, map[string]string{
		"projects/my-project/secrets/creds/versions/1": `{"username":"admin"}`,
	})

	_, err := p.Retrieve(t.Context(),
		schemeName+":projects/my-project/secrets/creds/versions/1#password", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "password")
	assert.Contains(t, err.Error(), "not found in secret JSON map")
}

func TestProvider_Retrieve_InvalidJSON(t *testing.T) {
	t.Parallel()
	p := newTestProvider(t, map[string]string{
		"projects/my-project/secrets/bad-json/versions/1": "not-a-json-string",
	})

	_, err := p.Retrieve(t.Context(),
		schemeName+":projects/my-project/secrets/bad-json/versions/1#somekey", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshalling")
}

// ── Lifecycle tests ───────────────────────────────────────────────────────────

func TestFactory(t *testing.T) {
	t.Parallel()
	p := NewFactory().Create(confmap.ProviderSettings{Logger: zap.NewNop()})
	_, ok := p.(*provider)
	require.True(t, ok)
}

func TestShutdown_ClosesClient(t *testing.T) {
	t.Parallel()
	mock := &mockSecretsManagerClient{}
	p := &provider{client: mock, logger: zap.NewNop()}

	require.False(t, mock.clientClosed)
	require.NoError(t, p.Shutdown(t.Context()))
	require.True(t, mock.clientClosed)
}

func TestShutdown_NilClientNoError(t *testing.T) {
	t.Parallel()
	p := &provider{client: nil, logger: zap.NewNop()}
	require.NoError(t, p.Shutdown(t.Context()))
}

func TestScheme(t *testing.T) {
	t.Parallel()
	p := &provider{logger: zap.NewNop()}
	assert.Equal(t, schemeName, p.Scheme())
}
