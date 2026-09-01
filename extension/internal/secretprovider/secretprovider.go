// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package secretprovider // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/internal/secretprovider"

import "context"

// SecretProvider is an interface that extensions can implement to provide
// secret values to other extensions (e.g., basicauth). The provider owns
// the refresh loop and caches the secret value internally.
//
// This interface enables a cloud-agnostic pattern: separate provider extensions
// (e.g., AWS Secrets Manager, GCP Secret Manager, Azure Key Vault, HashiCorp Vault)
// implement SecretProvider, and consumer extensions reference them by component ID
// via host.GetExtensions().
type SecretProvider interface {
	// GetSecret returns the current cached secret value.
	GetSecret(ctx context.Context) (string, error)

	// OnChange registers a callback that the provider calls from its own
	// refresh goroutine whenever the secret value changes. Only one callback
	// is supported per provider instance.
	OnChange(fn func(newValue string))
}
