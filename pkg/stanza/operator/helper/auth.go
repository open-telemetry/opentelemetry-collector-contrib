// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configauth"
	"go.opentelemetry.io/collector/extension/extensionauth"
)

var errNoHost = errors.New("an authenticator is configured but no host is available to resolve it")

// AuthConfig configures an optional server authenticator for connection-based
// stanza inputs, such as the tcp input. It is meant to be embedded by operators
// that accept network connections so they can delegate authentication to a
// standard extensionauth.Server extension.
type AuthConfig struct {
	configauth.Config `mapstructure:",squash"`
}

// IsConfigured reports whether an authenticator was requested.
func (c *AuthConfig) IsConfigured() bool {
	return c.AuthenticatorID != (component.ID{})
}

// GetServer resolves the configured server authenticator from the host's
// extensions. It returns a nil server when no authenticator is configured.
func (c *AuthConfig) GetServer(ctx context.Context, host component.Host) (extensionauth.Server, error) {
	if !c.IsConfigured() {
		return nil, nil
	}
	if host == nil {
		return nil, errNoHost
	}
	return c.GetServerAuthenticator(ctx, host.GetExtensions())
}
