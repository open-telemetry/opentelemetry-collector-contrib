// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package configdbauth implements the configuration settings for sourcing a
// connection credential from a db_auth provider extension. Unlike configauth,
// which supplies transport-level authentication for HTTP/gRPC requests, this
// package wires a connection-oriented component (such as a database receiver) to
// a credential provider that supplies a username/secret at connection-open time,
// with support for credentials that expire.
//
// A consuming component embeds Config under a "db_auth" key. The value is the
// component ID of a provider extension declared in the extensions block:
//
//	db_auth: aws_iam
//
// A named instance selects a specific declared extension:
//
//	db_auth: aws_iam/primary
//
// The named extension must be declared in the extensions block and implement
// dbauth.Provider; all provider-wide configuration (region, ...) lives on the
// extension's own config. To vary that configuration per consumer, declare
// multiple extension instances and reference the one you want. At Start the
// component calls Config.GetProvider with the host extension map to resolve the
// dbauth.Provider, mirroring how config/configauth resolves an authenticator
// extension via Config.GetServerAuthenticator.
package configdbauth // import "github.com/open-telemetry/opentelemetry-collector-contrib/config/configdbauth"

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/dbauth"
)

var (
	errNoCredentials = errors.New("db_auth: no credential provider configured")
	errNoExtension   = errors.New("db_auth: requested credential provider is not present")
	errNotProvider   = errors.New("db_auth: requested extension is not a credential provider")
)

// Config wires a connection-oriented component to a credential provider
// extension. A component embeds it under a "db_auth" key whose value is the
// provider extension's component ID:
//
//	db_auth: aws_iam
//
// The named extension must be declared in the extensions block and implement
// dbauth.Provider. GetProvider resolves it from the host extension map.
type Config struct {
	// ProviderID is the component ID of the credential provider extension. It is
	// populated from the scalar db_auth value (e.g. "aws_iam", or a named instance
	// "aws_iam/primary") via UnmarshalText, not from a mapstructure key, so it is
	// tagged "-" to be skipped by mapstructure decode and config-struct validation.
	ProviderID component.ID `mapstructure:"-"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// UnmarshalText lets the db_auth block decode from a scalar component ID (e.g.
// "aws_iam"). confmap's text-unmarshaler decode hook invokes this whenever the
// db_auth value is a string, exactly as it does for a bare component.ID field.
func (c *Config) UnmarshalText(text []byte) error {
	return c.ProviderID.UnmarshalText(text)
}

// IsEmpty reports whether no provider is configured. A component treats an empty
// Config as "db_auth not in use" and falls back to its existing static credential
// fields — the framework is opt-in.
func (c Config) IsEmpty() bool {
	return c.ProviderID == component.ID{}
}

// GetProvider resolves the configured credential provider from the host extension
// map (component.Host.GetExtensions()). It is called from a consuming component's
// Start, mirroring configauth.Config.GetServerAuthenticator.
//
// It returns an error when no provider is configured, when the named extension is
// not present, or when the named extension does not implement dbauth.Provider.
func (c Config) GetProvider(extensions map[component.ID]component.Component) (dbauth.Provider, error) {
	if c.IsEmpty() {
		return nil, errNoCredentials
	}

	ext, found := extensions[c.ProviderID]
	if !found {
		return nil, fmt.Errorf("%w: %q", errNoExtension, c.ProviderID)
	}
	provider, ok := ext.(dbauth.Provider)
	if !ok {
		return nil, fmt.Errorf("%w: %q", errNotProvider, c.ProviderID)
	}
	return provider, nil
}
