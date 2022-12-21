// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package asapauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/asapauthextension"

import (
	"context"
	"fmt"
	"net/http"

	"bitbucket.org/atlassian/go-asap/v2"
	"github.com/SermoDigital/jose/crypto"
	"go.opentelemetry.io/collector/extension/auth"
	"google.golang.org/grpc/credentials"
)

func createASAPClientAuthenticator(cfg *Config) (auth.Client, error) {
	pk, err := asap.NewPrivateKey([]byte(cfg.PrivateKey))
	if err != nil {
		return nil, err
	}

	// Caching provisioner will only issue a new token after the current token's expiry (determined by TTL).
	p := asap.NewCachingProvisioner(asap.NewProvisioner(
		cfg.KeyID, cfg.TTL, cfg.Issuer, cfg.Audience, crypto.SigningMethodRS256))

	return auth.NewClient(
		auth.WithClientRoundTripper(func(base http.RoundTripper) (http.RoundTripper, error) {
			return asap.NewTransportDecorator(p, pk)(base), nil
		}),
		auth.WithClientPerRPCCredentials(func() (credentials.PerRPCCredentials, error) {
			return &perRPCAuth{provisioner: p, privateKey: pk}, nil
		}),
	), nil
}

// perRPCAuth is a gRPC credentials.PerRPCCredentials implementation that returns an 'authorization' header.
type perRPCAuth struct {
	provisioner asap.Provisioner
	privateKey  interface{}
}

// GetRequestMetadata returns the request metadata to be used with the RPC.
func (c *perRPCAuth) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	token, err := c.provisioner.Provision()
	if err != nil {
		return nil, err
	}
	headerValue, err := token.Serialize(c.privateKey)
	if err != nil {
		return nil, err
	}
	header := map[string]string{
		"authorization": fmt.Sprintf("Bearer %s", string(headerValue)),
	}
	return header, nil
}

// RequireTransportSecurity always returns true for this implementation.
func (*perRPCAuth) RequireTransportSecurity() bool {
	return true
}
