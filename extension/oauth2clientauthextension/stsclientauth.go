package oauth2clientauthextension

import (
	"context"
	"fmt"
	"net/http"

	"go.opentelemetry.io/collector/component"
	"google.golang.org/grpc/credentials"

	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

// stsClientAuthenticator implements the Oauth2 STS Token Exchange protocol
type stsClientAuthenticator struct {
	config      *Config
	logger      *zap.Logger
	tokenSource oauth2.TokenSource
	transport   http.RoundTripper
	component.StartFunc
	component.ShutdownFunc
}

var _ clientAuthenticator = (*stsClientAuthenticator)(nil)

func newStsClientAuthenticator(cfg *Config, logger *zap.Logger) (clientAuthenticator, error) {
	transport, err := createTransport(cfg)
	if err != nil {
		return nil, err
	}
	ts, err := newStsTokenSource(cfg, transport)
	if err != nil {
		return nil, fmt.Errorf("failed to create sts token source: %v", err)
	}
	return &stsClientAuthenticator{
		config:      cfg,
		logger:      logger,
		tokenSource: ts,
		transport:   transport,
	}, nil
}

func (o *stsClientAuthenticator) RoundTripper(base http.RoundTripper) (http.RoundTripper, error) {
	return &oauth2.Transport{
		Source: errorWrappingTokenSource{
			ts:       o.tokenSource,
			tokenURL: o.config.TokenURL,
		},
		Base: base,
	}, nil
}

// Use custom implementation instead of grpcOauth.TokenSource so we are able to disable Transport Security
func (o *stsClientAuthenticator) PerRPCCredentials() (credentials.PerRPCCredentials, error) {
	return o, nil
}

func (o *stsClientAuthenticator) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	tok, err := o.tokenSource.Token()
	if err != nil {
		return nil, err
	}
	return map[string]string{
		"authorization": fmt.Sprintf("Bearer %s", tok.AccessToken),
	}, nil
}

// Applied when authenticator is returned as PerRPCCredentials
func (o *stsClientAuthenticator) RequireTransportSecurity() bool {
	return !o.config.DisablegRPCTransportSecurity
}
