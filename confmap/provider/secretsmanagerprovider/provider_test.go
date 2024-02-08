// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package secretsmanagerprovider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/confmap"
)

// Create a provider mocking s3provider works in normal cases
func NewTestProvider(url string) confmap.Provider {
	cfg := aws.NewConfig()
	cfg.EndpointResolverWithOptions = aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			PartitionID:   "aws",
			URL:           url,
			SigningRegion: "us-east-1",
		}, nil
	})

	return &provider{client: secretsmanager.NewFromConfig(*cfg)}
}

func TestSecretsManagerFetchSecret(t *testing.T) {
	secretName := "FOO"
	secretValue := "BAR"

	s := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-Amz-Target") == "secretsmanager.GetSecretValue" {
			response := &struct {
				Arn          string `json:"ARN"`
				CreatedDate  int64  `json:"CreatedDate"`
				Name         string `json:"Name"`
				SecretString string `json:"SecretString"`
			}{
				Arn:          secretName,
				CreatedDate:  time.Now().Unix(),
				Name:         secretName,
				SecretString: secretValue,
			}

			b, _ := json.Marshal(response)
			writer.Write(b)
			writer.WriteHeader(http.StatusOK)
		}
	}))
	defer s.Close()
	fp := NewTestProvider(s.URL)
	result, err := fp.Retrieve(context.Background(), "secretsmanager:"+secretName, nil)
	assert.NoError(t, err)
	assert.NoError(t, fp.Shutdown(context.Background()))

	value, err := result.AsRaw()
	assert.NoError(t, err)
	assert.NotNil(t, value)
	assert.Equal(t, secretValue, value)
}
