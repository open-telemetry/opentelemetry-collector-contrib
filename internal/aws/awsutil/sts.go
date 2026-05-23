// Copyright The OpenTelemetry Authors
// Portions of this file Copyright 2018-2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package awsutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	override "github.com/amazon-contributing/opentelemetry-collector-contrib/override/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	ststypes "github.com/aws/aws-sdk-go-v2/service/sts/types"
	smithymiddleware "github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// Confused Deputy Prevention header keys and the env vars that drive
// them. See https://docs.aws.amazon.com/IAM/latest/UserGuide/confused-deputy.html.
const (
	SourceArnHeaderKey     = "x-amz-source-arn"
	SourceAccountHeaderKey = "x-amz-source-account"
	AmzSourceAccount       = "AMZ_SOURCE_ACCOUNT"
	AmzSourceArn           = "AMZ_SOURCE_ARN"
)

// stsCredentialsProvider falls back from a regional STS endpoint to the
// partition's primary endpoint on RegionDisabledException, and latches
// onto the partitional client for all subsequent retrievals.
type stsCredentialsProvider struct {
	regional, partitional, fallback aws.CredentialsProvider
}

var _ aws.CredentialsProvider = (*stsCredentialsProvider)(nil)

func (p *stsCredentialsProvider) Retrieve(ctx context.Context) (aws.Credentials, error) {
	if p.fallback != nil {
		return p.fallback.Retrieve(ctx)
	}
	creds, err := p.regional.Retrieve(ctx)
	if err != nil {
		var rde *ststypes.RegionDisabledException
		if errors.As(err, &rde) && p.partitional != nil {
			p.fallback = p.partitional
			return p.fallback.Retrieve(ctx)
		}
	}
	return creds, err
}

// newStsCredentialsProvider returns a provider that assumes roleARN
// against region's STS endpoint, with automatic fallback to the
// partition's primary STS endpoint on RegionDisabledException.
//
// externalID is threaded into stscreds.AssumeRoleOptions when non-empty.
//
// If region cannot be resolved to a known partition, the partitional
// client is not constructed; the provider behaves as regional-only and
// surfaces the regional error rather than retrying in the wrong
// partition.
func newStsCredentialsProvider(cfg aws.Config, roleARN, region, externalID string) aws.CredentialsProvider {
	regionalCfg := cfg.Copy()
	regionalCfg.Region = region

	opts := func(o *stscreds.AssumeRoleOptions) {
		if externalID != "" {
			o.ExternalID = &externalID
		}
	}

	p := &stsCredentialsProvider{
		regional: stscreds.NewAssumeRoleProvider(newAssumeRoleClient(regionalCfg), roleARN, opts),
	}

	if fallback := override.GetPartitionPrimaryRegion(region); fallback != "" {
		partitionalCfg := cfg.Copy()
		partitionalCfg.Region = fallback
		p.partitional = stscreds.NewAssumeRoleProvider(newAssumeRoleClient(partitionalCfg), roleARN, opts)
	}

	return p
}

// newAssumeRoleClient is overrideable in tests.
var newAssumeRoleClient = newStsClient

// newStsClient builds an STS client. When both AmzSourceAccount and
// AmzSourceArn env vars are non-empty, registers a Build/Before
// middleware that stamps the Confused Deputy headers on every request.
// Build/Before runs ahead of SigV4 signing, so the headers are part of
// the signed request.
func newStsClient(cfg aws.Config) stscreds.AssumeRoleAPIClient {
	var options []func(*sts.Options)

	sourceAccount := os.Getenv(AmzSourceAccount)
	sourceArn := os.Getenv(AmzSourceArn)
	if sourceAccount != "" && sourceArn != "" {
		options = append(options, func(o *sts.Options) {
			o.APIOptions = append(o.APIOptions, func(s *smithymiddleware.Stack) error {
				return s.Build.Add(newCustomHeaderMiddleware("ConfusedDeputyHeaders", map[string]string{
					SourceArnHeaderKey:     sourceArn,
					SourceAccountHeaderKey: sourceAccount,
				}), smithymiddleware.Before)
			})
		})
		log.Printf("I! Found confused deputy header environment variables: source account: %q, source arn: %q",
			sourceAccount, sourceArn)
	}

	return sts.NewFromConfig(cfg, options...)
}

// customHeaderMiddleware sets a fixed set of HTTP headers on outgoing
// requests during the smithy Build step.
type customHeaderMiddleware struct {
	id      string
	headers map[string]string
}

var _ smithymiddleware.BuildMiddleware = (*customHeaderMiddleware)(nil)

func newCustomHeaderMiddleware(id string, headers map[string]string) *customHeaderMiddleware {
	return &customHeaderMiddleware{id: id, headers: headers}
}

func (m *customHeaderMiddleware) ID() string { return m.id }

func (m *customHeaderMiddleware) HandleBuild(
	ctx context.Context,
	in smithymiddleware.BuildInput,
	next smithymiddleware.BuildHandler,
) (smithymiddleware.BuildOutput, smithymiddleware.Metadata, error) {
	req, ok := in.Request.(*smithyhttp.Request)
	if !ok {
		return smithymiddleware.BuildOutput{}, smithymiddleware.Metadata{},
			fmt.Errorf("unrecognized transport type %T", in.Request)
	}
	for k, v := range m.headers {
		req.Header.Set(k, v)
	}
	return next.HandleBuild(ctx, in)
}
