// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clientauth // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/googleclientauthextension/internal/clientauth"

import (
	"errors"

	"go.opentelemetry.io/collector/component"
)

const (
	// accessToken indicates OAuth 2.0 access token (https://cloud.google.com/docs/authentication/token-types#access)
	accessToken = "access_token"

	// idToken indicates Google-signed ID-token (https://cloud.google.com/docs/authentication/token-types#id)
	idToken = "id_token"

	authorizationHeader      = "authorization"
	proxyAuthorizationHeader = "proxy-authorization"
)

var tokenTypes = map[string]struct{}{
	accessToken: {},
	idToken:     {},
}

var tokenHeaders = map[string]struct{}{
	authorizationHeader:      {},
	proxyAuthorizationHeader: {},
}

// Config stores the configuration for GCP Client Credentials.
type Config struct {
	// Project is the project telemetry is sent to if the gcp.project.id
	// resource attribute is not set. If unspecified, this is determined using
	// application default credentials.
	Project string `mapstructure:"project"`

	// QuotaProject specifies a project for quota and billing purposes. The
	// caller must have serviceusage.services.use permission on the project.
	//
	// For more information please read:
	// https://cloud.google.com/apis/docs/system-parameters
	QuotaProject string `mapstructure:"quota_project"`

	// TokenType specifies which type of token will be generated.
	// default: access_token
	TokenType string `mapstructure:"token_type,omitempty"`

	// Audience specifies the audience claim used for generating ID token.
	Audience string `mapstructure:"audience,omitempty"`

	// TokenHeader controls which HTTP header carries the token.
	// "authorization" (default) or "proxy-authorization" (for IAP-protected endpoints).
	TokenHeader string `mapstructure:"token_header,omitempty"`

	// Scope specifies optional requested permissions.
	// See https://datatracker.ietf.org/doc/html/rfc6749#section-3.3
	Scopes []string `mapstructure:"scopes,omitempty"`

	// TODO: Support impersonation, similar to what exists in the googlecloud collector exporter.
}

var _ component.Config = (*Config)(nil)

// Validate checks if the extension configuration is valid.
func (cfg *Config) Validate() error {
	if _, ok := tokenTypes[cfg.TokenType]; !ok {
		return errors.New("invalid token_type")
	}

	if cfg.TokenType == idToken && cfg.Audience == "" {
		return errors.New("audience must be specified when using the id_token token_type")
	}

	if _, ok := tokenHeaders[cfg.TokenHeader]; !ok {
		return errors.New("invalid token_header, must be \"authorization\" or \"proxy-authorization\"")
	}

	return nil
}

func CreateDefaultConfig() component.Config {
	return &Config{
		Scopes: []string{
			"https://www.googleapis.com/auth/cloud-platform",
			"https://www.googleapis.com/auth/logging.write",
			"https://www.googleapis.com/auth/monitoring.write",
			"https://www.googleapis.com/auth/trace.append",
		},
		TokenType:   accessToken,
		TokenHeader: authorizationHeader,
	}
}
