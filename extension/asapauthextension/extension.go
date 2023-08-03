// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
