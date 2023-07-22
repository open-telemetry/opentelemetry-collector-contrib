// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oidcauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/oidcauthextension"

// Config has the configuration for the OIDC Authenticator extension.
type Config struct {

	// The attribute (header name) to look for auth data. Optional, default value: "authorization".
	Attribute string `mapstructure:"attribute"`

	// IssuerURL is the base URL for the OIDC provider.
	// Required.
	IssuerURL string `mapstructure:"issuer_url"`

	// Audience of the token, used during the verification.
	// For example: "https://accounts.google.com" or "https://login.salesforce.com".
	// Required.
	Audience string `mapstructure:"audience"`

	// The local path for the issuer CA's TLS server cert.
	// Optional.
	IssuerCAPath string `mapstructure:"issuer_ca_path"`

	// The claim to use as the username, in case the token's 'sub' isn't the suitable source.
	// Optional.
	UsernameClaim string `mapstructure:"username_claim"`

	// The claim that holds the subject's group membership information.
	// Optional.
	GroupsClaim string `mapstructure:"groups_claim"`
}
