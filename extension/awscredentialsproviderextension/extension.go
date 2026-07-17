// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package awscredentialsproviderextension // import "github.com/elastic/opentelemetry-collector-components/extension/awscredentialsproviderextension"

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
)

// Provider is the interface that AWS-SDK-based components use to obtain credentials
// from this extension. Components look the extension up via host.GetExtensions() using
// the component ID they were configured with, and type-assert to this interface (or to
// a structurally identical local interface, to avoid a module dependency).
type Provider interface {
	// GetCredentialsProvider returns the credentials provider resolved by the
	// extension. It is only valid after the extension has been started.
	GetCredentialsProvider() aws.CredentialsProvider
}

type awsCredentialsProviderExtension struct {
	cfg   *Config
	creds aws.CredentialsProvider

	component.ShutdownFunc
}

var (
	_ extension.Extension = (*awsCredentialsProviderExtension)(nil)
	_ Provider            = (*awsCredentialsProviderExtension)(nil)
)

func newAWSCredentialsProviderExtension(cfg *Config) *awsCredentialsProviderExtension {
	return &awsCredentialsProviderExtension{cfg: cfg}
}

func (e *awsCredentialsProviderExtension) Start(ctx context.Context, _ component.Host) error {
	creds, err := buildCredentialsProvider(ctx, e.cfg)
	if err != nil {
		return err
	}
	e.creds = creds
	return nil
}

func (e *awsCredentialsProviderExtension) GetCredentialsProvider() aws.CredentialsProvider {
	return e.creds
}

// buildCredentialsProvider resolves the credentials provider from the configuration:
// static credentials when set, otherwise the default SDK chain (optionally narrowed by
// profile), with STS role assumption layered on top when configured.
func buildCredentialsProvider(ctx context.Context, cfg *Config) (aws.CredentialsProvider, error) {
	opts := []func(*awsconfig.LoadOptions) error{}
	if cfg.IMDSEndpoint != "" {
		opts = append(opts, awsconfig.WithEC2IMDSEndpoint(cfg.IMDSEndpoint))
	}
	if cfg.Profile != "" {
		opts = append(opts, awsconfig.WithSharedConfigProfile(cfg.Profile))
	}
	if creds := cfg.Credentials.Get(); creds != nil {
		opts = append(opts, awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			creds.AccessKeyID,
			string(creds.SecretAccessKey),
			string(creds.SessionToken),
		)))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, err
	}

	if role := cfg.AssumeRole.Get(); role != nil {
		stsClient := sts.NewFromConfig(awsCfg, func(o *sts.Options) {
			if role.STSRegion != "" {
				o.Region = role.STSRegion
			}
		})
		provider := stscreds.NewAssumeRoleProvider(stsClient, role.ARN,
			func(o *stscreds.AssumeRoleOptions) {
				if role.ExternalID != "" {
					o.ExternalID = aws.String(role.ExternalID)
				}
				if role.SessionName != "" {
					o.RoleSessionName = role.SessionName
				}
			})
		return aws.NewCredentialsCache(provider), nil
	}

	return awsCfg.Credentials, nil
}
