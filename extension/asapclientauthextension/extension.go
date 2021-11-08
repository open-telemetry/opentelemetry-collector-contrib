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

package asapclientauthextension

import (
	asap "bitbucket.org/atlassian/go-asap"
	"context"
	"github.com/SermoDigital/jose/crypto"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configauth"
	"google.golang.org/grpc/credentials"
	"net/http"
	"time"
)

// AsapClientAuthenticator implements ClientAuthenticator
type AsapClientAuthenticator struct {
	provisioner asap.Provisioner
	privateKey interface{}
}

func (a AsapClientAuthenticator) RoundTripper(base http.RoundTripper) (http.RoundTripper, error) {
	// needs provisioner and private key
	return asap.NewTransportDecorator(a.provisioner, a.privateKey)(base), nil
}

func (a AsapClientAuthenticator) PerRPCCredentials() (credentials.PerRPCCredentials, error) {
	panic("implement me")
}

func (a AsapClientAuthenticator) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (a AsapClientAuthenticator) Shutdown(_ context.Context) error {
	return nil
}

var _ configauth.ClientAuthenticator = (*AsapClientAuthenticator)(nil)

func createAsapClientAuthenticator(settings component.ExtensionCreateSettings, cfg *Config) (AsapClientAuthenticator, error) {
	var a AsapClientAuthenticator
	pk, err := asap.NewPrivateKey([]byte(cfg.PrivateKey))
	if err != nil {
		return a, err
	}
	a = AsapClientAuthenticator{
		provisioner: asap.NewProvisioner(
			cfg.KeyId, time.Duration(cfg.Ttl)*time.Second, cfg.Issuer, cfg.Audience, crypto.SigningMethodRS256),
		privateKey: pk,
	}
	return a, nil
}
