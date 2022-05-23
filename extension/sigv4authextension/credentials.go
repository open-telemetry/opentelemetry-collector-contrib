// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sigv4authextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/sigv4authextension"

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go-v2/service/sts/types"
)

// ProviderName provides a name of AssumeChainedRole provider
const ProviderName = "AssumeChainedRoleProvider"

// DefaultDuration is the default amount of time in minutes that the
// credentials will be valid for. This value is only used by AssumeRoleProvider
// for specifying the default expiry duration of an assume role.
//
// Other providers such as WebIdentityRoleProvider do not use this value, and
// instead rely on STS API's default parameter handing to assign a default
// value.
var DefaultDuration = time.Duration(15) * time.Minute

// AssumeRoleProvider retrieves temporary credentials from the STS service, and
// keeps track of their expiration time.
//
// This credential provider will be used by the SDKs default credential change
// when shared configuration is enabled, and the shared config or shared credentials
// file configure assume role. See Session docs for how to do this.
//
// AssumeChainedRoleProvider does not provide any synchronization and it is not safe
// to share this value across multiple Credentials, Sessions, or service clients
// without also sharing the same Credentials instance.
//
// Adapted from `stscreds.AssumeRoleProvider`.
type AssumeChainedRoleProvider struct {
	options AssumeChainedRoleOptions
}

// AssumeChainedRoleOptions is the configurable options for AssumeChainedRoleProvider
type AssumeChainedRoleOptions struct {
	// Client implementation of the AssumeRole operation. Required
	Client stscreds.AssumeRoleAPIClient

	// IAM Role ARNs to be assumed. Required
	RoleARNs []string

	// Session name, if you wish to uniquely identify this session.
	RoleSessionName string

	// Expiry duration of the STS credentials. Defaults to 15 minutes if not set.
	Duration time.Duration

	// Optional ExternalID to pass along, defaults to nil if not set.
	ExternalID *string

	// The policy plain text must be 2048 bytes or shorter. However, an internal
	// conversion compresses it into a packed binary format with a separate limit.
	// The PackedPolicySize response element indicates by percentage how close to
	// the upper size limit the policy is, with 100% equaling the maximum allowed
	// size.
	Policy *string

	// The ARNs of IAM managed policies you want to use as managed session policies.
	// The policies must exist in the same account as the role.
	//
	// This parameter is optional. You can provide up to 10 managed policy ARNs.
	// However, the plain text that you use for both inline and managed session
	// policies can't exceed 2,048 characters.
	//
	// An AWS conversion compresses the passed session policies and session tags
	// into a packed binary format that has a separate limit. Your request can fail
	// for this limit even if your plain text meets the other requirements. The
	// PackedPolicySize response element indicates by percentage how close the policies
	// and tags for your request are to the upper size limit.
	//
	// Passing policies to this operation returns new temporary credentials. The
	// resulting session's permissions are the intersection of the role's identity-based
	// policy and the session policies. You can use the role's temporary credentials
	// in subsequent AWS API calls to access resources in the account that owns
	// the role. You cannot use session policies to grant more permissions than
	// those allowed by the identity-based policy of the role that is being assumed.
	// For more information, see Session Policies (https://docs.aws.amazon.com/IAM/latest/UserGuide/access_policies.html#policies_session)
	// in the IAM User Guide.
	PolicyARNs []types.PolicyDescriptorType

	// The identification number of the MFA device that is associated with the user
	// who is making the AssumeRole call. Specify this value if the trust policy
	// of the role being assumed includes a condition that requires MFA authentication.
	// The value is either the serial number for a hardware device (such as GAHT12345678)
	// or an Amazon Resource Name (ARN) for a virtual device (such as arn:aws:iam::123456789012:mfa/user).
	SerialNumber *string

	// The source identity specified by the principal that is calling the AssumeRole
	// operation. You can require users to specify a source identity when they assume a
	// role. You do this by using the sts:SourceIdentity condition key in a role trust
	// policy. You can use source identity information in CloudTrail logs to determine
	// who took actions with a role. You can use the aws:SourceIdentity condition key
	// to further control access to Amazon Web Services resources based on the value of
	// source identity. For more information about using source identity, see Monitor
	// and control actions taken with assumed roles
	// (https://docs.aws.amazon.com/IAM/latest/UserGuide/id_credentials_temp_control-access_monitor.html)
	// in the IAM User Guide.
	SourceIdentity *string

	// Async method of providing MFA token code for assuming an IAM role with MFA.
	// The value returned by the function will be used as the TokenCode in the Retrieve
	// call. See StdinTokenProvider for a provider that prompts and reads from stdin.
	//
	// This token provider will be called when ever the assumed role's
	// credentials need to be refreshed when SerialNumber is set.
	TokenProvider func() (string, error)

	// A list of session tags that you want to pass. Each session tag consists of a key
	// name and an associated value. For more information about session tags, see
	// Tagging STS Sessions
	// (https://docs.aws.amazon.com/IAM/latest/UserGuide/id_session-tags.html) in the
	// IAM User Guide. This parameter is optional. You can pass up to 50 session tags.
	Tags []types.Tag

	// A list of keys for session tags that you want to set as transitive. If you set a
	// tag key as transitive, the corresponding key and value passes to subsequent
	// sessions in a role chain. For more information, see Chaining Roles with Session
	// Tags
	// (https://docs.aws.amazon.com/IAM/latest/UserGuide/id_session-tags.html#id_session-tags_role-chaining)
	// in the IAM User Guide. This parameter is optional.
	TransitiveTagKeys []string
}

// NewAssumeRoleProvider constructs and returns a credentials provider that
// will retrieve credentials by assuming a IAM role using STS.
func NewAssumeChainedRoleProvider(client stscreds.AssumeRoleAPIClient, roleARNs []string, optFns ...func(*AssumeChainedRoleOptions)) *AssumeChainedRoleProvider {
	o := AssumeChainedRoleOptions{
		Client:   client,
		RoleARNs: roleARNs,
	}

	for _, fn := range optFns {
		fn(&o)
	}

	return &AssumeChainedRoleProvider{
		options: o,
	}
}

func (p *AssumeChainedRoleProvider) Retrieve(ctx context.Context) (aws.Credentials, error) {
	var (
		provider aws.CredentialsProvider
		creds    aws.Credentials
		err      error
	)
	for _, roleARN := range p.options.RoleARNs {
		creds, err = p.retrieveChain(ctx, roleARN, provider)
		if err != nil {
			return aws.Credentials{}, err
		}
		provider = credentials.StaticCredentialsProvider{Value: creds}
	}
	return creds, err
}

// Retrieve generates a new set of temporary credentials using STS.
func (p *AssumeChainedRoleProvider) retrieveChain(ctx context.Context, roleARN string, provider aws.CredentialsProvider) (aws.Credentials, error) {
	// Apply defaults where parameters are not set.
	if len(p.options.RoleSessionName) == 0 {
		// Try to work out a role name that will hopefully end up unique.
		p.options.RoleSessionName = fmt.Sprintf("aws-go-sdk-%d", time.Now().UTC().UnixNano())
	}
	if p.options.Duration == 0 {
		// Expire as often as AWS permits.
		p.options.Duration = DefaultDuration
	}
	input := &sts.AssumeRoleInput{
		DurationSeconds:   aws.Int32(int32(p.options.Duration / time.Second)),
		PolicyArns:        p.options.PolicyARNs,
		RoleArn:           aws.String(roleARN),
		RoleSessionName:   aws.String(p.options.RoleSessionName),
		ExternalId:        p.options.ExternalID,
		SourceIdentity:    p.options.SourceIdentity,
		Tags:              p.options.Tags,
		TransitiveTagKeys: p.options.TransitiveTagKeys,
	}
	if p.options.Policy != nil {
		input.Policy = p.options.Policy
	}
	if p.options.SerialNumber != nil {
		if p.options.TokenProvider != nil {
			input.SerialNumber = p.options.SerialNumber
			code, err := p.options.TokenProvider()
			if err != nil {
				return aws.Credentials{}, err
			}
			input.TokenCode = aws.String(code)
		} else {
			return aws.Credentials{}, fmt.Errorf("assume role with MFA enabled, but TokenProvider is not set")
		}
	}

	withProvider := func(o *sts.Options) {
		if provider != nil {
			o.Credentials = provider
		}
	}
	resp, err := p.options.Client.AssumeRole(ctx, input, withProvider)
	if err != nil {
		return aws.Credentials{Source: ProviderName}, err
	}

	return aws.Credentials{
		AccessKeyID:     *resp.Credentials.AccessKeyId,
		SecretAccessKey: *resp.Credentials.SecretAccessKey,
		SessionToken:    *resp.Credentials.SessionToken,
		Source:          ProviderName,

		CanExpire: true,
		Expires:   *resp.Credentials.Expiration,
	}, nil
}
