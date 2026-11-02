// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsiamdbauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/dbauth/awsiamdbauthextension"

import "errors"

// Config is the aws_iam_db_auth provider extension's config. It carries the provider-wide
// inputs an operator sets on the extension:
//
//	extensions:
//	  aws_iam_db_auth:
//	    region: us-east-1   # required
//
// Region is required. The per-connection mint inputs — the database endpoint and
// user — travel with each GetCredential call as a dbauth.Request, sourced from the
// consuming component's own endpoint and configured username.
type Config struct {
	// Region is the AWS region of the database. Required: a token cannot be minted
	// without it, so it is validated at config load.
	Region string `mapstructure:"region"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// errNoRegion is returned at config load when the extension has no region set.
var errNoRegion = errors.New("aws_iam_db_auth: region must be set on the extension")

// Validate fails when no region is configured.
func (c *Config) Validate() error {
	if c.Region == "" {
		return errNoRegion
	}
	return nil
}
