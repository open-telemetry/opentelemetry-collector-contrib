// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package parameterstoreprovider

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
)

// Mock AWS SSM
type testSSMClient struct {
	name      string
	value     string
	encrypted bool
}

// Implement GetParameter()
func (client *testSSMClient) GetParameter(_ context.Context, params *ssm.GetParameterInput, _ ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
	if client.encrypted && !*params.WithDecryption {
		return nil, errors.New("attempt to read encrypted parameter without decryption")
	}

	if client.name != *params.Name {
		return nil, fmt.Errorf("unexpected parameter name; expected: %q got: %q", client.name, *params.Name)
	}

	return &ssm.GetParameterOutput{
		Parameter: &types.Parameter{
			Value: aws.String(client.value),
		},
	}, nil
}

// Create a provider using mock ssm client
func NewTestProvider(name, value string, encrypted bool) confmap.Provider {
	return &provider{client: &testSSMClient{name: name, value: value, encrypted: encrypted}}
}

func TestFetchParameterStorePlain(t *testing.T) {
	parameterValue := "BAR"

	for testName, nameAndURI := range uriAndNameVariants("") {
		t.Run(testName, func(t *testing.T) {
			fp := NewTestProvider(nameAndURI[0], parameterValue, false)
			result, err := fp.Retrieve(context.Background(), nameAndURI[1], nil)

			assert.NoError(t, err)
			assert.NoError(t, fp.Shutdown(context.Background()))

			value, err := result.AsRaw()
			assert.NoError(t, err)
			assert.NotNil(t, value)
			assert.Equal(t, parameterValue, value)
		})
	}
}

func TestFetchParameterStoreEncrypted(t *testing.T) {
	parameterValue := "BAR"

	for testName, nameAndURI := range uriAndNameVariants("?withDecryption=true") {
		t.Run(testName, func(t *testing.T) {
			fp := NewTestProvider(nameAndURI[0], parameterValue, false)
			result, err := fp.Retrieve(context.Background(), nameAndURI[1], nil)

			assert.NoError(t, err)
			assert.NoError(t, fp.Shutdown(context.Background()))

			value, err := result.AsRaw()
			assert.NoError(t, err)
			assert.NotNil(t, value)
			assert.Equal(t, parameterValue, value)
		})
	}
}

func TestFetchParameterStoreEncryptedWithoutDecryption(t *testing.T) {
	parameterValue := "BAR"

	for testName, nameAndURI := range uriAndNameVariants("") {
		t.Run(testName, func(t *testing.T) {
			fp := NewTestProvider(nameAndURI[0], parameterValue, true)
			_, err := fp.Retrieve(context.Background(), nameAndURI[1], nil)

			assert.Error(t, err)
			assert.NoError(t, fp.Shutdown(context.Background()))
		})
	}
}

func TestFetchParameterStoreFieldValidJson(t *testing.T) {
	parameterValue := "BAR"
	parameterJSON := fmt.Sprintf("{\"field1\": \"%s\"}", parameterValue)

	for testName, nameAndURI := range uriAndNameVariants("#field1") {
		t.Run(testName, func(t *testing.T) {
			fp := NewTestProvider(nameAndURI[0], parameterJSON, false)
			result, err := fp.Retrieve(context.Background(), nameAndURI[1], nil)

			assert.NoError(t, err)
			assert.NoError(t, fp.Shutdown(context.Background()))

			value, err := result.AsRaw()
			assert.NoError(t, err)
			assert.NotNil(t, value)
			assert.Equal(t, parameterValue, value)
		})
	}
}

func TestFetchParameterStoreFieldValidEncryptedJson(t *testing.T) {
	parameterValue := "BAR"
	parameterJSON := fmt.Sprintf("{\"field1\": \"%s\"}", parameterValue)

	for testName, nameAndURI := range uriAndNameVariants("?withDecryption=true#field1") {
		t.Run(testName, func(t *testing.T) {
			fp := NewTestProvider(nameAndURI[0], parameterJSON, true)
			result, err := fp.Retrieve(context.Background(), nameAndURI[1], nil)

			assert.NoError(t, err)
			assert.NoError(t, fp.Shutdown(context.Background()))

			value, err := result.AsRaw()
			assert.NoError(t, err)
			assert.NotNil(t, value)
			assert.Equal(t, parameterValue, value)
		})
	}
}

func TestFetchParameterStoreFieldInvalidJson(t *testing.T) {
	parameterValue := "BAR"

	for testName, nameAndURI := range uriAndNameVariants("#field1") {
		t.Run(testName, func(t *testing.T) {
			fp := NewTestProvider(nameAndURI[0], parameterValue, true)
			_, err := fp.Retrieve(context.Background(), nameAndURI[1], nil)

			assert.Error(t, err)
			assert.NoError(t, fp.Shutdown(context.Background()))
		})
	}
}

func TestFetchParameterStoreFieldMissingInJson(t *testing.T) {
	parameterValue := "BAR"
	parameterJSON := fmt.Sprintf("{\"field0\": \"%s\"}", parameterValue)

	for testName, nameAndURI := range uriAndNameVariants("#field1") {
		t.Run(testName, func(t *testing.T) {
			fp := NewTestProvider(nameAndURI[0], parameterJSON, false)
			_, err := fp.Retrieve(context.Background(), nameAndURI[1], nil)

			assert.Error(t, err)
			assert.NoError(t, fp.Shutdown(context.Background()))
		})
	}
}

func TestFactory(t *testing.T) {
	p := NewFactory().Create(confmap.ProviderSettings{})
	_, ok := p.(*provider)
	require.True(t, ok)
}

func uriAndNameVariants(uriSuffix string) map[string][2]string {
	name := "/test/parameter"
	arn := "arn:aws:ssm:us-east-1:123456789012:parameter:" + name

	return map[string][2]string{
		"name": {name, "parameterstore:" + name + uriSuffix},
		"arn":  {arn, "parameterstore:" + arn + uriSuffix},
	}
}
