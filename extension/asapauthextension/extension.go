// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package asapauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/asapauthextension"

import (
	"context"
	"fmt"
	"net/http"

	"bitbucket.org/atlassian/go-asap/v2"
	"github.com/SermoDigital/jose/crypto"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension/extensionauth"
	"google.golang.org/grpc/credentials"
)

var _ extensionauth.Client = (*asapAuthExtension)(nil)

type asapAuthExtension struct {
	component.StartFunc
	component.ShutdownFunc

	provisioner asap.Provisioner
	privateKey  any
}

// PerRPCCredentials returns extensionauth.Client.
func (e *asapAuthExtension) PerRPCCredentials() (credentials.PerRPCCredentials, error) {
	return &perRPCAuth{provisioner: e.provisioner, privateKey: e.privateKey}, nil
}

// RoundTripper implements extensionauth.Client.
func (e *asapAuthExtension) RoundTripper(base http.RoundTripper) (http.RoundTripper, error) {
	return asap.NewTransportDecorator(e.provisioner, e.privateKey)(base), nil
}

func createASAPClientAuthenticator(cfg *Config) (extensionauth.Client, error) {
	pk, err := asap.NewPrivateKey([]byte(cfg.PrivateKey))
	if err != nil {
		return nil, err
	}

	// Caching provisioner will only issue a new token after the current token's expiry (determined by TTL).
	p := asap.NewCachingProvisioner(asap.NewProvisioner(
		cfg.KeyID, cfg.TTL, cfg.Issuer, cfg.Audience, crypto.SigningMethodRS256))

	return &asapAuthExtension{
		provisioner: p,
		privateKey:  pk,
	}, nil
}

// perRPCAuth is a gRPC credentials.PerRPCCredentials implementation that returns an 'authorization' header.
type perRPCAuth struct {
	provisioner asap.Provisioner
	privateKey  any
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
