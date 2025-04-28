// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package googlesecretsprovider // import "github.com/open-telemetry/opentelemetry-collector-contrib/confmap/provider/googlesecretsprovider"

import (
	"context"
	"fmt"
	"strings"

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
	schemeName = "googlesecretsprovider"
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
		return nil, fmt.Errorf("%q uri is not supported by %q provider", uri, schemeName)
	}
	secretName := strings.TrimPrefix(uri, schemeName+":")

	if p.client == nil {
		client, err := secretmanager.NewClient(ctx)
		if err != nil {

			return nil, fmt.Errorf("failed to create a Google secret manager client: %w", err)
		}
		defer client.Close()
		p.client = client
	}
	req := &secretmanagerpb.AccessSecretVersionRequest{
		Name: secretName,
	}
	resp, err := p.client.AccessSecretVersion(ctx, req)

	if err != nil {
		var apiErr *apierror.APIError
		apiErr, ok := apierror.FromError(err)
		if !ok {
			return nil, fmt.Errorf("failed to access secret version: %w", err)
		}
		return nil, fmt.Errorf("failed to access secret version: %v", apiErr.Error())
	}

	return confmap.NewRetrieved(string(resp.GetPayload().GetData()))
}

func (*provider) Scheme() string {
	return schemeName
}

func (*provider) Shutdown(context.Context) error {
	return nil
}
