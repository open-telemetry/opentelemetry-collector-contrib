// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clientauth // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/googleclientauthextension/internal/clientauth"

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig_Validate_ValidAccessToken(t *testing.T) {
	cfg := &Config{
		TokenType:   accessToken,
		TokenHeader: authorizationHeader,
	}

	err := cfg.Validate()
	assert.NoError(t, err)
}

func TestConfig_Validate_ValidIDToken(t *testing.T) {
	cfg := &Config{
		TokenType:   idToken,
		Audience:    "audience",
		TokenHeader: authorizationHeader,
	}

	err := cfg.Validate()
	assert.NoError(t, err)
}

func TestConfig_Validate_MissingAudience(t *testing.T) {
	cfg := &Config{
		TokenType:   idToken,
		TokenHeader: authorizationHeader,
	}

	err := cfg.Validate()
	assert.Error(t, err)
}

func TestConfig_Validate_Invalid(t *testing.T) {
	cfg := &Config{
		TokenType:   "invalid",
		TokenHeader: authorizationHeader,
	}

	err := cfg.Validate()
	assert.Error(t, err)
}

func TestConfig_Validate_ProxyAuthorizationHeader(t *testing.T) {
	cfg := &Config{
		TokenType:   accessToken,
		TokenHeader: proxyAuthorizationHeader,
	}

	err := cfg.Validate()
	assert.NoError(t, err)
}

func TestConfig_Validate_InvalidTokenHeader(t *testing.T) {
	cfg := &Config{
		TokenType:   accessToken,
		TokenHeader: "invalid",
	}

	err := cfg.Validate()
	assert.Error(t, err)
}

func TestConfig_Validate_EmptyTokenHeader(t *testing.T) {
	cfg := &Config{
		TokenType:   accessToken,
		TokenHeader: "",
	}

	err := cfg.Validate()
	assert.Error(t, err)
}

func TestConfig_DefaultTokenHeader(t *testing.T) {
	cfg := CreateDefaultConfig().(*Config)
	assert.Equal(t, authorizationHeader, cfg.TokenHeader)
}
