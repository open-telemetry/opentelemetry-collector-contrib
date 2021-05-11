// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package utils

import (
	"go.uber.org/zap"
	"gopkg.in/zorkian/go-datadog-api.v2"
)

// CreateClient creates a new Datadog client
func CreateClient(APIKey string, endpoint string) *datadog.Client {
	client := datadog.NewClient(APIKey, "")
	client.SetBaseUrl(endpoint)

	return client
}

// ValidateAPIKey checks that the provided client was given a correct API key.
func ValidateAPIKey(logger *zap.Logger, client *datadog.Client) {
	logger.Info("Validating API key.")
	res, err := client.Validate()
	if err != nil {
		logger.Warn("Error while validating API key.", zap.Error(err))
	}

	if res {
		logger.Info("API key validation successful.")
	} else {
		logger.Warn("API key validation failed.")
	}
}
