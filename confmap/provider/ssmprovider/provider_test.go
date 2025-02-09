// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ssmprovider

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
)

// Mock AWS SSM client
type testSSMClient struct {
	parameterValue string
}

// Implement GetParameter()
func (client *testSSMClient) GetParameter(_ context.Context, _ *ssm.GetParameterInput,
	_ ...func(*ssm.Options),
) (*ssm.GetParameterOutput, error) {
	return &ssm.GetParameterOutput{Parameter: &ssm.Parameter{Value: &client.parameterValue}}, nil
}

// Create a provider using mock SSM client
func NewTestProvider(parameterValue string) confmap.Provider {
	return &provider{client: &testSSMClient{parameterValue: parameterValue}}
}

func TestSSMProviderFetchParameter(t *testing.T) {
	paramName := "FOO"
	paramValue := "BAR"

	fp := NewTestProvider(paramValue)
	result, err := fp.Retrieve(context.Background(), "ssm:"+paramName, nil)

	assert.NoError(t, err)
	assert.NoError(t, fp.Shutdown(context.Background()))

	value, err := result.AsRaw()
	assert.NoError(t, err)
	assert.NotNil(t, value)
	assert.Equal(t, paramValue, value)
}

func TestSSMProviderFetchJsonKey(t *testing.T) {
	paramName := "FOO#field1"
	paramValue := "BAR"
	jsonParam := fmt.Sprintf("{\"field1\": \"%s\"}", paramValue)

	fp := NewTestProvider(jsonParam)
	result, err := fp.Retrieve(context.Background(), "ssm:"+paramName, nil)

	assert.NoError(t, err)
	assert.NoError(t, fp.Shutdown(context.Background()))

	value, err := result.AsRaw()
	assert.NoError(t, err)
	assert.NotNil(t, value)
	assert.Equal(t, paramValue, value)
}

func TestSSMProviderInvalidURI(t *testing.T) {
	fp := NewTestProvider("dummy")
	_, err := fp.Retrieve(context.Background(), "invalid:FOO", nil)

	assert.Error(t, err)
	assert.NoError(t, fp.Shutdown(context.Background()))
}

func TestFactory(t *testing.T) {
	p := NewFactory().Create(confmap.ProviderSettings{})
	_, ok := p.(*provider)
	require.True(t, ok)
}
