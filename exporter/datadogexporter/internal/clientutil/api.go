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

package clientutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/clientutil"

import (
	"errors"

	"go.uber.org/zap"
	zorkian "gopkg.in/zorkian/go-datadog-api.v2"
)

// CreateZorkianClient creates a new Zorkian Datadog client
func CreateZorkianClient(apiKey string, endpoint string) *zorkian.Client {
	client := zorkian.NewClient(apiKey, "")
	client.SetBaseUrl(endpoint)

	return client
}

var ErrInvalidAPI = errors.New("API Key validation failed")

// ValidateAPIKeyZorkian checks that the provided client was given a correct API key.
func ValidateAPIKeyZorkian(logger *zap.Logger, client *zorkian.Client) error {
	logger.Info("Validating API key.")
	valid, err := client.Validate()
	if err == nil && valid {
		logger.Info("API key validation successful.")
		return nil
	}
	if err != nil {
		logger.Warn("Error while validating API key", zap.Error(err))
		return nil
	}
	logger.Warn(ErrInvalidAPI.Error())
	return ErrInvalidAPI
}
