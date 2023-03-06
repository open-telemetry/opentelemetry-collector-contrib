// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package secretsmanagerprovider

import (
	"context"
	"encoding/json"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/confmap"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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
