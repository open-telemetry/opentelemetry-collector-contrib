// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package dbauth defines the interface a db_auth provider extension implements.
// Unlike extensionauth, which supplies transport-level authentication for
// outgoing HTTP/gRPC requests, a db_auth provider supplies a credential
// (username/secret) to a connection-oriented component (such as a database
// receiver) at connection-open time, with support for credentials that expire.
//
// A provider is a Collector extension that also implements Provider. It is
// declared in the extensions block and referenced by a consuming component
// through configdbauth.ID; the component resolves it from the host extension
// map at Start, mirroring how config/configauth resolves an authenticator
// extension.
package dbauth // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/dbauth"

import (
	"context"
	"time"
)

// Credential is the value a Provider returns. The same shape is used by every
// provider: a credential carries optional material plus an optional expiry, and
// nothing about how that material should be applied — the consuming component
// decides that.
type Credential struct {
	// Username is the username the consumer should use. A nil pointer means the
	// provider does not supply a username and the consumer should fall back to its
	// own configured username; a non-nil pointer (including a pointer to the empty
	// string) means the provider generated the username and the consumer must use
	// it. The nil-vs-empty distinction is load-bearing, so this is a pointer.
	Username *string

	// Secret is the opaque secret material — a password or a minted token. It is
	// placed wherever the consuming driver expects the secret (e.g. the password
	// slot of a connection string). It is never logged.
	Secret string

	// NotAfter is an advisory hint for when the credential expires. A nil pointer
	// means no expiry applies (e.g. a static password). It is advisory only: the
	// provider owns refresh-before-expiry, so consumers need not act on it, but it
	// is useful for observability.
	NotAfter *time.Time

	// prevent unkeyed literal initialization
	_ struct{}
}

// Request carries the per-connection inputs a consumer supplies on every
// GetCredential call. Provider-wide configuration (such as an AWS region) belongs
// in the provider extension's own config; only the connection-specific values that
// the consumer already knows travel in the Request, so an operator never repeats
// the endpoint or username they configured on the component. A provider ignores
// the fields it does not need.
type Request struct {
	// Endpoint is the connection endpoint, in host:port form, the credential is
	// for. A token-minting provider uses it as a mint input (e.g. the RDS endpoint
	// an AWS IAM token is scoped to).
	Endpoint string

	// Username is the consumer's configured username. A provider may use it as a
	// mint input (e.g. the database user an IAM token authenticates) or ignore it.
	Username string

	// prevent unkeyed literal initialization
	_ struct{}
}

// Provider supplies a Credential to a connection-oriented component. A provider
// is a Collector extension that also implements this interface. The provider
// owns any caching and refresh-before-expiry; a consumer may call GetCredential
// on every connection-open and trust the returned credential is currently valid.
type Provider interface {
	// GetCredential returns the credential valid at the time of the call for the
	// given request.
	//
	// It must honor context cancellation and be safe for concurrent use — a
	// connection pool may call it from many goroutines.
	GetCredential(ctx context.Context, req Request) (*Credential, error)
}
