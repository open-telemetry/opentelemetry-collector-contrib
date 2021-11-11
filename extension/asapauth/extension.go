// Copyright  The OpenTelemetry Authors
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

package asapauth

import (
	"context"
	"fmt"
	"net/http"
	"time"

	asap "bitbucket.org/atlassian/go-asap"
	"github.com/SermoDigital/jose/crypto"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configauth"
	"google.golang.org/grpc/credentials"
)

// AsapClientAuthenticator implements ClientAuthenticator
type AsapClientAuthenticator struct {
	provisioner asap.Provisioner
	privateKey  interface{}
}

func (a AsapClientAuthenticator) RoundTripper(base http.RoundTripper) (http.RoundTripper, error) {
	return asap.NewTransportDecorator(a.provisioner, a.privateKey)(base), nil
}

func (a AsapClientAuthenticator) PerRPCCredentials() (credentials.PerRPCCredentials, error) {
	token, err := a.provisioner.Provision()
	if err != nil {
		return nil, err
	}
	headerValue, err := token.Serialize(a.privateKey)
	if err != nil {
		return nil, err
	}
	header := map[string]string{
		"authorization": fmt.Sprintf("Bearer %s", string(headerValue)),
	}
	return &PerRPCAuth{
		metadata: header,
	}, nil
}

// Start does nothing and returns nil
func (a AsapClientAuthenticator) Start(_ context.Context, _ component.Host) error {
	return nil
}

// Shutdown does nothing and returns nil
func (a AsapClientAuthenticator) Shutdown(_ context.Context) error {
	return nil
}

var _ configauth.ClientAuthenticator = (*AsapClientAuthenticator)(nil)

func createAsapClientAuthenticator(cfg *Config) (AsapClientAuthenticator, error) {
	var a AsapClientAuthenticator
	pk, err := asap.NewPrivateKey([]byte(cfg.PrivateKey))
	if err != nil {
		return a, err
	}
	a = AsapClientAuthenticator{
		provisioner: asap.NewCachingProvisioner(asap.NewProvisioner(
			cfg.KeyID, time.Duration(cfg.TTL)*time.Second, cfg.Issuer, cfg.Audience, crypto.SigningMethodRS256)),
		privateKey: pk,
	}
	return a, nil
}

var _ credentials.PerRPCCredentials = (*PerRPCAuth)(nil)

// PerRPCAuth is a gRPC credentials.PerRPCCredentials implementation that returns an 'authorization' header.
type PerRPCAuth struct {
	metadata map[string]string
}

// GetRequestMetadata returns the request metadata to be used with the RPC.
func (c *PerRPCAuth) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return c.metadata, nil
}

// RequireTransportSecurity always returns true for this implementation.
func (*PerRPCAuth) RequireTransportSecurity() bool {
	return true
}
