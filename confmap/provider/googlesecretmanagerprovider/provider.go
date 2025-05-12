// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package googlesecretmanagerprovider // import "github.com/open-telemetry/opentelemetry-collector-contrib/confmap/provider/googlesecretmanagerprovider"

import (
	"context"
	"errors"
	"fmt"
	"strings"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	gax "github.com/googleapis/gax-go/v2"
	"github.com/googleapis/gax-go/v2/apierror"
	"go.opentelemetry.io/collector/confmap"
)

type secretsManagerClient interface {
	AccessSecretVersion(ctx context.Context, req *secretmanagerpb.AccessSecretVersionRequest, opts ...gax.CallOption) (*secretmanagerpb.AccessSecretVersionResponse, error)
	Close() error
}

const (
	schemeName = "googlesecretmanager"
)

var (
	ErrURINotSupported     = errors.New("uri is not supported by Google Secret Manager Provider")
	ErrAccessSecretVersion = errors.New("failed to access secret version")
)

type provider struct {
	client secretsManagerClient
}

func NewFactory() confmap.ProviderFactory {
	return confmap.NewProviderFactory(newProvider)
}

func newProvider(confmap.ProviderSettings) confmap.Provider {
	return &provider{client: nil}
}

func (p *provider) Retrieve(ctx context.Context, uri string, _ confmap.WatcherFunc) (*confmap.Retrieved, error) {
	if !strings.HasPrefix(uri, schemeName+":") {
		return nil, fmt.Errorf("%q: %w", uri, ErrURINotSupported)
	}
	secretName := strings.TrimPrefix(uri, schemeName+":")
	if p.client == nil {
		client, err := secretmanager.NewClient(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to create a Google secret manager client: %w", err)
		}
		p.client = client
	}
	req := &secretmanagerpb.AccessSecretVersionRequest{
		Name: secretName,
	}
	resp, err := p.client.AccessSecretVersion(ctx, req)
	if err != nil {
		apiErr, ok := apierror.FromError(err)

		if !ok {
			return nil, fmt.Errorf("%w : %w", ErrAccessSecretVersion, err)
		}
		return nil, fmt.Errorf("%w: %v", ErrAccessSecretVersion, apiErr.Error())
	}
	return confmap.NewRetrieved(string(resp.GetPayload().GetData()))
}

func (*provider) Scheme() string {
	return schemeName
}

func (p *provider) Shutdown(context.Context) error {
	if p.client != nil {
		p.client.Close()
	}
	return nil
}
