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
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	sigv4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"go.opentelemetry.io/collector/component/componenthelper"
	"go.opentelemetry.io/collector/config/configauth"
	"go.uber.org/zap"
	grpcCredentials "google.golang.org/grpc/credentials"
)

// Sigv4Auth is a struct that implements the configauth.ClientAuthenticator interface.
// It provides the implementation for providing Sigv4 authentication for HTTP requests only.
type Sigv4Auth struct {
	cfg                          *Config
	logger                       *zap.Logger
	awsSDKInfo                   string
	componenthelper.StartFunc    // embedded default behavior to do nothing with Start()
	componenthelper.ShutdownFunc // embedded default behavior to do nothing with Shutdown()
}

// compile time check that the Sigv4Auth struct satisfies the configauth.ClientAuthenticator interface
var _ configauth.ClientAuthenticator = (*Sigv4Auth)(nil)

// RoundTripper() returns a custom SigningRoundTripper.
func (sa *Sigv4Auth) RoundTripper(base http.RoundTripper) (http.RoundTripper, error) {
	cfg := sa.cfg

	signer := sigv4.NewSigner()

	// Create the SigningRoundTripper struct
	rt := SigningRoundTripper{
		transport:  base,
		signer:     signer,
		region:     cfg.Region,
		service:    cfg.Service,
		creds:      cfg.creds,
		awsSDKInfo: sa.awsSDKInfo,
		logger:     sa.logger,
	}

	return &rt, nil
}

// PerRPCCredentials() is implemented to satisfy the configauth.ClientAuthenticator
// interface but will not be implemented.
func (sa *Sigv4Auth) PerRPCCredentials() (grpcCredentials.PerRPCCredentials, error) {
	return nil, errors.New("Not Implemented")
}

// newSigv4Extension() is called by createExtension() in factory.go and
// returns a new Sigv4Auth struct.
func newSigv4Extension(cfg *Config, awsSDKInfo string, logger *zap.Logger) *Sigv4Auth {
	return &Sigv4Auth{
		cfg:        cfg,
		logger:     logger,
		awsSDKInfo: awsSDKInfo,
	}
}

// getCredsFromConfig() is a helper function that gets AWS credentials
// from the Config.
func getCredsFromConfig(cfg *Config) (*aws.Credentials, error) {
	awscfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(cfg.Region),
	)
	if err != nil {
		return nil, err
	}
	if cfg.RoleArn != "" {
		stsSvc := sts.NewFromConfig(awscfg)
		provider := stscreds.NewAssumeRoleProvider(stsSvc, cfg.RoleArn, func(o *stscreds.AssumeRoleOptions) {
			o.RoleSessionName = "otel-" + strconv.FormatInt(time.Now().Unix(), 10)
		})
		creds, err := provider.Retrieve(context.Background())
		if err != nil {
			return nil, err
		}
		return &creds, nil
	}
	// Get Credentials, either from ./aws or from environmental variables.
	creds, err := awscfg.Credentials.Retrieve(context.Background())
	if err != nil {
		return nil, err
	}
	return &creds, nil
}

// cloneRequest() is a helper function that makes a shallow copy of the request and a
// deep copy of the header, for thread safety purposes.
func cloneRequest(r *http.Request) *http.Request {
	// shallow copy of the struct
	r2 := new(http.Request)
	*r2 = *r
	// deep copy of the Header
	r2.Header = make(http.Header, len(r.Header))
	for k, s := range r.Header {
		r2.Header[k] = append([]string(nil), s...)
	}
	return r2
}
