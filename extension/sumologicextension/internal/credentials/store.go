// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package credentials // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/sumologicextension/internal/credentials"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/sumologicextension/internal/api"
)

// CollectorCredentials are used for storing the credentials received during
// collector registration.
type CollectorCredentials struct {
	// CollectorName indicates what name was set in the configuration when
	// registration has been made.
	CollectorName string                          `json:"collectorName"`
	Credentials   api.OpenRegisterResponsePayload `json:"collectorCredentials"`
	// APIBaseURL saves the destination API base URL which was used for registration.
	// This is used for instance when the API redirects the collector to a different
	// deployment due to the fact that the installation token being used for registration
	// belongs to a different deployment.
	// In order to make collector registration work, we save the destination
	// API base URL so that when the collector starts up again it can use this
	// API base URL for communication with the backend.
	APIBaseURL string `json:"apiBaseUrl"`
}

// Store is an interface to get collector authentication data
type Store interface {
	// Check checks if collector credentials exist under the specified key.
	Check(key string) bool

	// Get returns the collector credentials stored under a specified key.
	Get(key string) (CollectorCredentials, error)

	// Store stores the provided collector credentials stored under a specified key.
	Store(key string, creds CollectorCredentials) error

	// Delete deletes collector credentials stored under the specified key.
	Delete(key string) error

	// Validate checks if the store is operating correctly
	Validate() error
}
