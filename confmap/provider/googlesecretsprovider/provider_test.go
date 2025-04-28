// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlesecretsprovider

import (
	"context"
	"errors"
	"testing"

	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	gax "github.com/googleapis/gax-go/v2"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
)

// Define a mock secretsManagerClient for testing
type mockSecretsManagerClient struct {
	err          error
	secretString string
}

func (m *mockSecretsManagerClient) AccessSecretVersion(ctx context.Context, req *secretmanagerpb.AccessSecretVersionRequest, opts ...gax.CallOption) (*secretmanagerpb.AccessSecretVersionResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &secretmanagerpb.AccessSecretVersionResponse{
		Payload: &secretmanagerpb.SecretPayload{
			Data: []byte(m.secretString),
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
			name: "Happy path: valid uri, secret entry exists and is accessible",
			uri:  schemeName + ":projects/my-project/secrets/test-secret-id/versions/1",
			testSecretManager: &mockSecretsManagerClient{
				err:          nil,
				secretString: "test-secret-value",
			},
			wantSecret: "test-secret-value",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			testProvider := &provider{
				client: tc.testSecretManager,
			}
			defer testProvider.client.Close()
			gotSecret, err := testProvider.Retrieve(context.Background(), tc.uri, nil)
			if err != nil {
				t.Errorf("%v: Retrieve() gotError = %v, want nil error", tc.name, err)
			}
			gotSecretString, err := gotSecret.AsString()
			if err != nil {
				t.Errorf("%v: failed to retrieve the string value of the secret, error: %v", tc.name, err)
			}
			if gotSecretString != tc.wantSecret {
				t.Errorf("%v: Retrieve() gotSecret = %v, want %v", tc.name, gotSecret, tc.wantSecret)
			}
		})
	}
}

func TestProvider_Retrieve_Failure(t *testing.T) {
	tests := []struct {
		name              string
		uri               string
		testSecretManager *mockSecretsManagerClient
	}{

		{
			name: "Invalid scheme",
			uri:  "invalidscheme" + ":projects/my-project/secrets/test-secret-id/versions/1",
			testSecretManager: &mockSecretsManagerClient{
				err:          errors.New("invalid scheme"),
				secretString: "test-secret-value",
			},
		},
		{
			name: "secret entry does not exist in the secret manager",
			uri:  schemeName + ":projects/my-project/secrets/non-existent/versions/1",
			testSecretManager: &mockSecretsManagerClient{
				err:          errors.New("secret entry does not exist"),
				secretString: "test-secret-value",
			},
		},
		{
			name: "invalid secret name",
			uri:  schemeName + ":projects/my-project/secrets-invalid/non-existent/versions-invalid/1",
			testSecretManager: &mockSecretsManagerClient{
				err:          errors.New("secret name is invalid"),
				secretString: "test-secret-value",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			testProvider := &provider{
				client: tc.testSecretManager,
			}
			defer testProvider.client.Close()
			_, err := testProvider.Retrieve(context.Background(), tc.uri, nil)
			if err == nil {
				t.Errorf("%v: Retrieve() got nil error, want non-nil error", tc.name)
			}
		})
	}
}
func TestFactory(t *testing.T) {
	p := NewFactory().Create(confmap.ProviderSettings{})
	_, ok := p.(*provider)
	require.True(t, ok)
}
