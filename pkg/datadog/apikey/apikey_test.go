// Copyright The OpenTelemetryAuthors
// SPDX-License-Identifier: Apache-2.0

package apikey

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"testing"

	"github.com/DataDog/datadog-agent/comp/otelcol/otlp/testutil"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.uber.org/zap/zaptest"
)

type validateAPIKeyResponse struct {
	Valid bool `json:"valid"`
}

func ValidateAPIKeyEndpointValid() (string, http.HandlerFunc) {
	return "/api/v1/validate", func(w http.ResponseWriter, _ *http.Request) {
		res := validateAPIKeyResponse{Valid: true}
		resJSON, _ := json.Marshal(res)

		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write(resJSON)
		if err != nil {
			log.Fatalln(err)
		}
	}
}

func TestStaticAPIKeyCheck(t *testing.T) {
	tests := []struct {
		name          string
		apiKey        string
		expectedError error
	}{
		{
			name:          "Valid API key",
			apiKey:        "1234567890abcdef",
			expectedError: nil,
		},
		{
			name:          "Empty API key",
			apiKey:        "",
			expectedError: ErrUnsetAPIKey,
		},
		{
			name:          "API key with invalid characters",
			apiKey:        "12345!@#$%",
			expectedError: ErrAPIKeyFormat,
		},
		{
			name:          "API key with mixed valid and invalid characters",
			apiKey:        "12345abcde!@#$%",
			expectedError: ErrAPIKeyFormat,
		},
		{
			name:          "API key with only invalid characters",
			apiKey:        "!@#$%",
			expectedError: ErrAPIKeyFormat,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := StaticAPIKeyCheck(tt.apiKey)
			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestFullAPIKeyCheck(t *testing.T) {
	tests := []struct {
		name                      string
		apiKeyValidServerResponse bool
		isMetricExportV2          bool
		failOnInvalidKey          bool
		expectedError             string
		expectedAPIClient         bool
		expectedZorkianClient     bool
	}{
		{
			name:                      "Valid API key with V2 enabled",
			apiKeyValidServerResponse: true,
			isMetricExportV2:          true,
			failOnInvalidKey:          true,
			expectedError:             "",
			expectedAPIClient:         true,
			expectedZorkianClient:     false,
		},
		{
			name:                      "Invalid API key with V2 enabled",
			apiKeyValidServerResponse: false,
			isMetricExportV2:          true,
			failOnInvalidKey:          true,
			expectedError:             "API Key validation failed",
			expectedAPIClient:         false,
			expectedZorkianClient:     false,
		},
		{
			name:                      "Valid API key with V2 disabled",
			apiKeyValidServerResponse: true,
			isMetricExportV2:          false,
			failOnInvalidKey:          true,
			expectedError:             "",
			expectedAPIClient:         false,
			expectedZorkianClient:     true,
		},
		{
			name:                      "Invalid API key with V2 disabled",
			apiKeyValidServerResponse: false,
			isMetricExportV2:          false,
			failOnInvalidKey:          true,
			expectedError:             "API Key validation failed",
			expectedAPIClient:         false,
			expectedZorkianClient:     false,
		},
		{
			name:                      "Valid API key with failOnInvalidKey false",
			apiKeyValidServerResponse: true,
			isMetricExportV2:          true,
			failOnInvalidKey:          false,
			expectedError:             "",
			expectedAPIClient:         true,
			expectedZorkianClient:     false,
		},
		{
			name:                      "Invalid API key with failOnInvalidKey false",
			apiKeyValidServerResponse: false,
			isMetricExportV2:          true,
			failOnInvalidKey:          false,
			expectedError:             "",
			expectedAPIClient:         true,
			expectedZorkianClient:     false,
		},
	}
	validServer := testutil.DatadogServerMock(ValidateAPIKeyEndpointValid)
	defer validServer.Close()
	invalidServer := testutil.DatadogServerMock(testutil.ValidateAPIKeyEndpointInvalid)
	defer invalidServer.Close()
	errchan := make(chan error, 1)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var server *testutil.DatadogServer
			if tt.apiKeyValidServerResponse {
				server = validServer
			} else {
				server = invalidServer
			}
			ctx := context.Background()
			logger := zaptest.NewLogger(t)
			buildInfo := component.BuildInfo{
				Command:     "otelcol",
				Description: "OpenTelemetry Collector",
				Version:     "1.0.0",
			}
			clientConfig := confighttp.ClientConfig{}

			apiClient, zorkianClient, err := FullAPIKeyCheck(
				ctx,
				"test-api-key",
				&errchan,
				buildInfo,
				tt.isMetricExportV2,
				tt.failOnInvalidKey,
				server.URL,
				logger,
				clientConfig,
			)

			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
			} else {
				assert.NoError(t, err)
			}

			if tt.expectedAPIClient {
				assert.NotNil(t, apiClient)
			} else {
				assert.Nil(t, apiClient)
			}

			if tt.expectedZorkianClient {
				assert.NotNil(t, zorkianClient)
			} else {
				assert.Nil(t, zorkianClient)
			}

			if !tt.failOnInvalidKey {
				<-errchan
			}
		})
	}
}
