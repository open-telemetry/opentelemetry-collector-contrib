// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

// Package oauth2clientauthextension implements `cauth.Client`
// This extension provides OAuth2 Client Credentials flow authenticator for HTTP and gRPC based exporters.
// The extension fetches and refreshes the token after expiry
// For further details about OAuth2 Client Credentials flow refer https://datatracker.ietf.org/doc/html/rfc6749#section-4.4
package oauth2clientauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/oauth2clientauthextension"
