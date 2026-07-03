// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package configdbauth implements the configuration settings for sourcing a
// connection credential from a db_auth provider extension. Unlike configauth,
// which supplies transport-level authentication for HTTP/gRPC requests, this
// package wires a connection-oriented component (such as a database receiver) to
// a credential provider that supplies a username/secret at connection-open time,
// with support for credentials that expire.
//
// A consuming component embeds Config under a "db_auth" key. The single key
// inside the block is the provider extension's component ID, and its value is an
// inline override of that extension's own config:
//
//	db_auth:
//	  aws_iam:
//	    region: us-east-1
package configdbauth // import "github.com/open-telemetry/opentelemetry-collector-contrib/config/configdbauth"

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/dbauth"
)

var (
	errNoCredentials     = errors.New("db_auth: no credential provider configured")
	errMultipleProviders = errors.New("db_auth: exactly one provider may be configured")
	errNoExtension       = errors.New("db_auth: requested credential provider is not present")
	errNotProvider       = errors.New("db_auth: requested extension is not a credential provider")
)

// Config wires a connection-oriented component to a credential provider
// extension. A component embeds it under a "db_auth" key; the single key inside
// the block is the provider extension's component ID and its value is an inline
// override of that extension's config:
//
//	db_auth:
//	  aws_iam:
//	    region: us-east-1
//
// The named extension must be declared in the extensions block and implement
// dbauth.Provider.
type Config struct {
	// ProviderConfigs holds the inline provider override, keyed by the provider
	// extension's component ID.
	ProviderConfigs map[string]any `mapstructure:",remain"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// IsEmpty reports whether no provider is configured. A component treats an empty
// Config as "db_auth not in use" and falls back to its existing static credential
// fields — the framework is opt-in.
func (c Config) IsEmpty() bool {
	return len(c.ProviderConfigs) == 0
}

// Validate fails when more than one provider is configured. Zero is allowed
// (opt-out); the unknown-provider and not-a-provider cases are reported by
// GetProvider, which is the only place the host extension map is known.
func (c Config) Validate() error {
	if len(c.ProviderConfigs) > 1 {
		return fmt.Errorf("%w, got %d", errMultipleProviders, len(c.ProviderConfigs))
	}
	return nil
}

// GetProvider resolves the configured credential provider from the host extension
// map (component.Host.GetExtensions()) and returns it along with the inline
// override (extensionArgs) the consumer supplied under the provider's ID.
//
// The returned extensionArgs is the raw value nested under the provider ID key
// (nil when the key has no body).
func (c Config) GetProvider(extensions map[component.ID]component.Component) (dbauth.Provider, map[string]any, error) {
	if err := c.Validate(); err != nil {
		return nil, nil, err
	}
	if c.IsEmpty() {
		return nil, nil, errNoCredentials
	}

	key, args := c.single()
	var id component.ID
	if err := id.UnmarshalText([]byte(key)); err != nil {
		return nil, nil, fmt.Errorf("db_auth: invalid provider id %q: %w", key, err)
	}

	ext, found := extensions[id]
	if !found {
		return nil, nil, fmt.Errorf("%w: %q", errNoExtension, id)
	}
	provider, ok := ext.(dbauth.Provider)
	if !ok {
		return nil, nil, fmt.Errorf("%w: %q", errNotProvider, id)
	}
	return provider, args, nil
}

// single returns the sole configured provider ID key and its inline override as a
// string map. Callers must ensure exactly one key is set (Validate + non-empty).
// The override is nil when the key has no value (e.g. "aws_iam:" with no body),
// meaning the provider uses its configured defaults unchanged.
func (c Config) single() (string, map[string]any) {
	for k, v := range c.ProviderConfigs {
		sub, _ := v.(map[string]any)
		return k, sub
	}
	return "", nil
}
