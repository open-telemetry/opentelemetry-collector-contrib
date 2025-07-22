// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oidcauthextension

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDuplicateIssuers(t *testing.T) {
	config := &Config{
		Attribute: "authorization",
		Providers: []ProviderCfg{
			{
				IssuerURL: "https://example.com",
				Audience:  "https://example.com",
			},
			{
				IssuerURL: "https://example.com",
				Audience:  "https://example.com",
			},
		},
	}
	require.Error(t, config.Validate())
}
