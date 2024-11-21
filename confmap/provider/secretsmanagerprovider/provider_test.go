// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package secretsmanagerprovider

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
)

// Mock AWS secretsmanager
type testSecretManagerClient struct {
	secretValue string
}

// Implement GetSecretValue()
func (client *testSecretManagerClient) GetSecretValue(_ context.Context, _ *secretsmanager.GetSecretValueInput,
	_ ...func(*secretsmanager.Options),
) (*secretsmanager.GetSecretValueOutput, error) {
	return &secretsmanager.GetSecretValueOutput{SecretString: &client.secretValue}, nil
}

// Create a provider using mock secretsmanager client
func NewTestProvider(secretValue string) confmap.Provider {
	return &provider{client: &testSecretManagerClient{secretValue: secretValue}}
}

func TestSecretsManagerFetchSecret(t *testing.T) {
	secretName := "FOO"
	secretValue := "BAR"

	fp := NewTestProvider(secretValue)
	result, err := fp.Retrieve(context.Background(), "secretsmanager:"+secretName, nil)

	assert.NoError(t, err)
	assert.NoError(t, fp.Shutdown(context.Background()))

	value, err := result.AsRaw()
	assert.NoError(t, err)
	assert.NotNil(t, value)
	assert.Equal(t, secretValue, value)
}

func TestFetchSecretsManagerFieldValidJson(t *testing.T) {
	secretName := "FOO#field1"
	secretValue := "BAR"
	secretJSON := fmt.Sprintf("{\"field1\": \"%s\"}", secretValue)

	fp := NewTestProvider(secretJSON)
	result, err := fp.Retrieve(context.Background(), "secretsmanager:"+secretName, nil)

	assert.NoError(t, err)
	assert.NoError(t, fp.Shutdown(context.Background()))

	value, err := result.AsRaw()
	assert.NoError(t, err)
	assert.NotNil(t, value)
	assert.Equal(t, secretValue, value)
}

func TestFetchSecretsManagerFieldInvalidJson(t *testing.T) {
	secretName := "FOO#field1"
	secretValue := "BAR"

	fp := NewTestProvider(secretValue)
	_, err := fp.Retrieve(context.Background(), "secretsmanager:"+secretName, nil)

	assert.Error(t, err)
	assert.NoError(t, fp.Shutdown(context.Background()))
}

func TestFetchSecretsManagerFieldMissingInJson(t *testing.T) {
	secretName := "FOO#field1"
	secretValue := "BAR"
	secretJSON := fmt.Sprintf("{\"field0\": \"%s\"}", secretValue)

	fp := NewTestProvider(secretJSON)
	_, err := fp.Retrieve(context.Background(), "secretsmanager:"+secretName, nil)

	assert.Error(t, err)
	assert.NoError(t, fp.Shutdown(context.Background()))
}

func TestFactory(t *testing.T) {
	p := NewFactory().Create(confmap.ProviderSettings{})
	_, ok := p.(*provider)
	require.True(t, ok)
}
