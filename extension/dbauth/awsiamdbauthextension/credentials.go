// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsiamdbauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/dbauth/awsiamdbauthextension"

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// assumeRoleCredentials returns cached credentials for ar.ARN, obtained through STS with the base config's credentials.
func assumeRoleCredentials(cfg aws.Config, ar AssumeRole) aws.CredentialsProvider {
	stsCfg := cfg.Copy()
	if ar.STSRegion != "" {
		stsCfg.Region = ar.STSRegion
	}
	return aws.NewCredentialsCache(stscreds.NewAssumeRoleProvider(sts.NewFromConfig(stsCfg), ar.ARN, func(o *stscreds.AssumeRoleOptions) {
		if ar.SessionName != "" {
			o.RoleSessionName = ar.SessionName
		}
		if ar.ExternalID != "" {
			o.ExternalID = aws.String(ar.ExternalID)
		}
	}))
}
