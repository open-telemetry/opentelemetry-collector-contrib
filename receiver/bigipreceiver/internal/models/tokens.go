// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package models // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/bigipreceiver/internal/models"

// TokenDetails represents the top level json returned by the authn/login endpoint
type TokenDetails struct {
	Token Token `json:"token"`
}

// Token represents where the actual token data is stored
type Token struct {
	Token string `json:"token,omitempty"`
}
