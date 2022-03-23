package honeycombauthextension

import (
	"context"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configauth"
	"google.golang.org/grpc/credentials"
	"net/http"
)

type PerRPCCredentials struct {
	metadata map[string]string
}

var _ credentials.PerRPCCredentials = (*PerRPCCredentials)(nil)

func (p *PerRPCCredentials) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return p.metadata, nil
}

func (p *PerRPCCredentials) RequireTransportSecurity() bool {
	return true
}

type ClientAuthenticator struct {
	team    string
	dataset string
}

var _ configauth.ClientAuthenticator = (*ClientAuthenticator)(nil)

func (ca *ClientAuthenticator) Start(context.Context, component.Host) error {
	return nil
}

func (ca *ClientAuthenticator) Shutdown(context.Context) error {
	return nil
}

func (ca *ClientAuthenticator) RoundTripper(base http.RoundTripper) (http.RoundTripper, error) {
	return base, nil
}

func (ca *ClientAuthenticator) PerRPCCredentials() (credentials.PerRPCCredentials, error) {
	return &PerRPCCredentials{
		metadata: map[string]string{
			teamMetadataKey:    ca.team,
			datasetMetadataKey: ca.dataset,
		},
	}, nil
}

func newClientAuthenticator(cfg *Config) *ClientAuthenticator {
	return &ClientAuthenticator{
		team:    cfg.Team,
		dataset: cfg.Dataset,
	}
}
