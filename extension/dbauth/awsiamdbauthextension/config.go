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

	// AssumeRole, when ARN is set, signs tokens with that role's credentials instead of
	// the default chain's. RDS only accepts tokens signed by a principal in the database's
	// account, so reaching another account's database needs a role there.
	AssumeRole AssumeRole `mapstructure:"assume_role"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// AssumeRole configures the role the extension assumes before minting tokens.
type AssumeRole struct {
	// ARN of the role to assume. Empty keeps the default credential chain.
	ARN string `mapstructure:"arn"`
	// SessionName is the role session name; the SDK generates one when empty.
	SessionName string `mapstructure:"session_name"`
	// STSRegion is the region of the STS endpoint; defaults to Region.
	STSRegion string `mapstructure:"sts_region"`
	// ExternalID is passed to AssumeRole when the role's trust policy requires one.
	ExternalID string `mapstructure:"external_id"`

	// prevent unkeyed literal initialization
	_ struct{}
}

var (
	// errNoRegion is returned at config load when the extension has no region set.
	errNoRegion = errors.New("aws_iam_db_auth: region must be set on the extension")
	// errAssumeRoleWithoutARN is returned when assume_role options are set without a role ARN.
	errAssumeRoleWithoutARN = errors.New("aws_iam_db_auth: assume_role.arn must be set when any other assume_role field is")
)

// Validate fails when no region is configured, or when assume_role options are set without an ARN.
func (c *Config) Validate() error {
	if c.Region == "" {
		return errNoRegion
	}
	ar := c.AssumeRole
	if ar.ARN == "" && (ar.SessionName != "" || ar.STSRegion != "" || ar.ExternalID != "") {
		return errAssumeRoleWithoutARN
	}
	return nil
}
