// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package googlesecretmanagerprovider // import "github.com/open-telemetry/opentelemetry-collector-contrib/confmap/provider/googlesecretmanagerprovider"

import (
	"context"
	"fmt"
	"strings"
	"sync"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	gax "github.com/googleapis/gax-go/v2"
	"github.com/googleapis/gax-go/v2/apierror"
	"go.opentelemetry.io/collector/confmap"

	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
)

type secretsManagerClient interface {
	AccessSecretVersion(ctx context.Context, req *secretmanagerpb.AccessSecretVersionRequest, opts ...gax.CallOption) (*secretmanagerpb.AccessSecretVersionResponse, error)
	Close() error
}

const (
	schemeName = "googlesecretmanagerprovider"
)

type provider struct {
	mu     sync.Mutex
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
		return nil, fmt.Errorf("%q uri is not supported by Google Secret Manager Provider", uri)
	}
	secretName := strings.TrimPrefix(uri, schemeName+":")
	p.mu.Lock()
	if p.client == nil {
		client, err := secretmanager.NewClient(ctx)
		if err != nil {
			p.mu.Unlock()
			return nil, fmt.Errorf("failed to create a Google secret manager client: %w", err)
		}
		p.client = client
	}
	p.mu.Unlock()
	req := &secretmanagerpb.AccessSecretVersionRequest{
		Name: secretName,
	}
	resp, err := p.client.AccessSecretVersion(ctx, req)
	if err != nil {
		apiErr, ok := apierror.FromError(err)
		errorMsg := "failed to access secret version"
		if !ok {
			return nil, fmt.Errorf(errorMsg+": %w", err)
		}
		return nil, fmt.Errorf(errorMsg+": %v", apiErr.Error())
	}
	return confmap.NewRetrieved(string(resp.GetPayload().GetData()))
}

func (*provider) Scheme() string {
	return schemeName
}

func (p *provider) Shutdown(context.Context) error {
	p.mu.Lock()
	if p.client != nil {
		p.client.Close()
	}
	p.mu.Unlock()
	return nil
}
