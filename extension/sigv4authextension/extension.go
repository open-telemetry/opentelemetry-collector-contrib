// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sigv4authextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/sigv4authextension"

import (
	"context"
	"fmt"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	sigv4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensionauth"
	"go.uber.org/zap"
)

// sigv4Auth is a struct that implements the extensionauth.HTTPClient interface.
// It provides the implementation for providing Sigv4 authentication for HTTP requests only.
type sigv4Auth struct {
	cfg                    *Config
	logger                 *zap.Logger
	awsSDKInfo             string
	component.StartFunc    // embedded default behavior to do nothing with Start()
	component.ShutdownFunc // embedded default behavior to do nothing with Shutdown()
}

// compile time check that the sigv4Auth struct satisfies the extensionauth.HTTPClient interface
var (
	_ extension.Extension      = (*sigv4Auth)(nil)
	_ extensionauth.HTTPClient = (*sigv4Auth)(nil)
)

// RoundTripper() returns a custom signingRoundTripper.
func (sa *sigv4Auth) RoundTripper(base http.RoundTripper) (http.RoundTripper, error) {
	cfg := sa.cfg

	signer := sigv4.NewSigner()

	// Create the signingRoundTripper struct
	rt := signingRoundTripper{
		transport:     base,
		signer:        signer,
		region:        cfg.Region,
		service:       cfg.Service,
		credsProvider: cfg.credsProvider,
		awsSDKInfo:    sa.awsSDKInfo,
		logger:        sa.logger,
	}

	return &rt, nil
}

// newSigv4Extension() is called by createExtension() in factory.go and
// returns a new sigv4Auth struct.
func newSigv4Extension(cfg *Config, awsSDKInfo string, logger *zap.Logger) *sigv4Auth {
	return &sigv4Auth{
		cfg:        cfg,
		logger:     logger,
		awsSDKInfo: awsSDKInfo,
	}
}

// getCredsProviderFromConfig() is a helper function that gets AWS credentials
// from the Config.
func getCredsProviderFromConfig(cfg *Config) (*aws.CredentialsProvider, error) {
	awscfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(cfg.AssumeRole.STSRegion),
	)
	if err != nil {
		return nil, err
	}
	if cfg.AssumeRole.ARN != "" {
		stsSvc := sts.NewFromConfig(awscfg)

		provider := stscreds.NewAssumeRoleProvider(stsSvc, cfg.AssumeRole.ARN)
		awscfg.Credentials = aws.NewCredentialsCache(provider)
	}

	_, err = awscfg.Credentials.Retrieve(context.Background())
	if err != nil {
		return nil, err
	}

	return &awscfg.Credentials, nil
}

func getCredsProviderFromWebIdentityConfig(cfg *Config) (*aws.CredentialsProvider, error) {
	tokenRetriever := stscreds.IdentityTokenRetriever(
		stscreds.IdentityTokenFile(cfg.AssumeRole.WebIdentityTokenFile),
	)
	_, err := tokenRetriever.GetIdentityToken()
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve token file: %w", err)
	}

	awscfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithWebIdentityRoleCredentialOptions(
			func(options *stscreds.WebIdentityRoleOptions) {
				options.TokenRetriever = tokenRetriever
				options.RoleARN = cfg.AssumeRole.ARN
			},
		),
		awsconfig.WithRegion(cfg.AssumeRole.STSRegion),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to load AWS configuration: %w", err)
	}
	stsSvc := sts.NewFromConfig(awscfg)

	provider := stscreds.NewWebIdentityRoleProvider(stsSvc, cfg.AssumeRole.ARN, tokenRetriever)
	awscfg.Credentials = aws.NewCredentialsCache(provider)

	return &awscfg.Credentials, nil
}
