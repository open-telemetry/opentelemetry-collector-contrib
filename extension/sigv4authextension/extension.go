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
	"errors"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	sigv4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configauth"
	"go.uber.org/zap"
	grpcCredentials "google.golang.org/grpc/credentials"
)

// sigv4Auth is a struct that implements the configauth.ClientAuthenticator interface.
// It provides the implementation for providing Sigv4 authentication for HTTP requests only.
type sigv4Auth struct {
	cfg                    *Config
	logger                 *zap.Logger
	awsSDKInfo             string
	component.StartFunc    // embedded default behavior to do nothing with Start()
	component.ShutdownFunc // embedded default behavior to do nothing with Shutdown()
}

// compile time check that the sigv4Auth struct satisfies the configauth.ClientAuthenticator interface
var _ configauth.ClientAuthenticator = (*sigv4Auth)(nil)

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

// PerRPCCredentials() is implemented to satisfy the configauth.ClientAuthenticator
// interface but will not be implemented.
func (sa *sigv4Auth) PerRPCCredentials() (grpcCredentials.PerRPCCredentials, error) {
	return nil, errors.New("Not Implemented")
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
