// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlesecretmanagerprovider

import (
	"context"
	"fmt"
	"testing"

	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	gax "github.com/googleapis/gax-go/v2"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
	"google.golang.org/grpc/codes"
)

var validSecrets = map[string]string{
	"projects/my-project/secrets/secret-1/versions/1": "secret-1",
	"projects/my-project/secrets/secret-2/versions/1": "secret-2",
}

// Define a mock secretsManagerClient for testing
type mockSecretsManagerClient struct{}

func (m *mockSecretsManagerClient) AccessSecretVersion(ctx context.Context, req *secretmanagerpb.AccessSecretVersionRequest, opts ...gax.CallOption) (*secretmanagerpb.AccessSecretVersionResponse, error) {
	secretString, ok := validSecrets[req.Name]
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
	return nil
}

func TestProvider_Retrieve_Success(t *testing.T) {
	tests := []struct {
		name              string
		uri               string
		testSecretManager *mockSecretsManagerClient
		wantSecret        string
	}{
		{
			name:              "Happy path: valid uri, secret entry exists and is accessible",
			uri:               schemeName + ":projects/my-project/secrets/secret-1/versions/1",
			testSecretManager: &mockSecretsManagerClient{},
			wantSecret:        "secret-1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			testProvider := &provider{
				client: tc.testSecretManager,
			}
			gotSecret, err := testProvider.Retrieve(context.Background(), tc.uri, nil)
			require.NoError(t, err)

			gotSecretString, err := gotSecret.AsString()
			require.NoError(t, err)
			require.Equal(t, tc.wantSecret, gotSecretString)
		})
	}
}

func TestProvider_Retrieve_Failure(t *testing.T) {
	tests := []struct {
		name string
		uri  string
	}{
		{
			name: "Invalid scheme",
			uri:  "invalidscheme" + ":projects/my-project/secrets/test-secret-id/versions/1",
		},
		{
			name: "secret entry does not exist in the secret manager",
			uri:  schemeName + ":projects/my-project/secrets/non-existent/versions/1",
		},
		{
			name: "invalid secret name",
			uri:  schemeName + ":projects/my-project/versions/1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			testProvider := &provider{
				client: &mockSecretsManagerClient{},
			}
			_, err := testProvider.Retrieve(context.Background(), tc.uri, nil)
			require.Error(t, err)
		})
	}
}

func TestFactory(t *testing.T) {
	p := NewFactory().Create(confmap.ProviderSettings{})
	_, ok := p.(*provider)
	require.True(t, ok)
}

func TestShutdown(t *testing.T) {
	tests := []struct {
		name              string
		testSecretManager *mockSecretsManagerClient
	}{
		{
			name:              "When secret manager client is non-nil",
			testSecretManager: &mockSecretsManagerClient{},
		},
		{
			name:              "When secret manager client is nil",
			testSecretManager: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			testProvider := &provider{
				client: tc.testSecretManager,
			}
			err := testProvider.Shutdown(context.Background())
			require.NoError(t, err)
			require.Nil(t, testProvider.client)
		})
	}
}
