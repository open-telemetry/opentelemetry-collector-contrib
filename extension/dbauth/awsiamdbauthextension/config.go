// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsiamdbauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/dbauth/awsiamdbauthextension"

import "errors"

// Config is the aws_iam provider extension's config. It carries the provider-wide
// inputs an operator sets on the extension:
//
//	extensions:
//	  aws_iam:
//	    region: us-east-1   # required
//	    # endpoint / db_user: optional, normally supplied by the receiver instead
//
// Region is required. The per-connection mint inputs — the database endpoint and
// user — normally travel with each GetCredential call as a dbauth.Request, sourced
// from the consuming component's own endpoint and configured username, so an
// operator never repeats them. Endpoint and DBUser exist for the cases where the
// operator wants to pin them explicitly; when set they are used as a fallback for
// any request that does not carry its own.
type Config struct {
	// Region is the AWS region of the database. Required: a token cannot be minted
	// without it, so it is validated at config load.
	Region string `mapstructure:"region"`

	// Endpoint, when set, is the database endpoint (host:port) the token is minted
	// for. Optional: the consuming receiver normally supplies its own endpoint with
	// each request. When the request carries no endpoint, this value is used.
	Endpoint string `mapstructure:"endpoint,omitempty"`

	// DBUser, when set, is the database user the token authenticates. Optional in the
	// same way as Endpoint: the receiver's configured username is used by default,
	// and this value is used only when the request carries no username.
	DBUser string `mapstructure:"db_user,omitempty"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// errNoRegion is returned at config load when the extension has no region set.
var errNoRegion = errors.New("aws_iam: region must be set on the extension")

// Validate fails when no region is configured. Region is the one required field;
// the endpoint and database user may be supplied by each consuming receiver.
func (c *Config) Validate() error {
	if c.Region == "" {
		return errNoRegion
	}
	return nil
}
