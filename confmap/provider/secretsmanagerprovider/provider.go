// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package secretsmanagerprovider // import "github.com/open-telemetry/opentelemetry-collector-contrib/confmap/provider/secretsmanagerprovider"

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"go.opentelemetry.io/collector/confmap"
)

const (
	schemeName = "secretsmanager"
)

type provider struct {
	client *secretsmanager.Client
}

// NewFactory returns a new confmap.ProviderFactory that creates a confmap.Provider
// which reads configuration the given AWS Secrets Manager Name or ARN.
//
// This Provider supports "secretsmanager" scheme, and can be called with a selector:
// `secretsmanager:NAME_OR_ARN`
func NewFactory() confmap.ProviderFactory {
	return confmap.NewProviderFactory(newWithSettings)
}

func newWithSettings(_ confmap.ProviderSettings) confmap.Provider {
	return &provider{client: nil}
}

// New returns a new confmap.Provider that reads the configuration from the given AWS Secrets Manager Name or ARN.
//
// This Provider supports "secretsmanager" scheme, and can be called with a selector:
// `secretsmanager:NAME_OR_ARN`
//
// Deprecated: [v0.100.0] Use NewFactory() instead.
func New() confmap.Provider {
	return &provider{}
}

func (provider *provider) Retrieve(ctx context.Context, uri string, _ confmap.WatcherFunc) (*confmap.Retrieved, error) {
	if !strings.HasPrefix(uri, schemeName+":") {
		return nil, fmt.Errorf("%q uri is not supported by %q provider", uri, schemeName)
	}

	secretArn := strings.Replace(uri, schemeName+":", "", 1)

	input := &secretsmanager.GetSecretValueInput{
		SecretId: &secretArn,
	}

	response, err := provider.client.GetSecretValue(ctx, input)
	if err != nil {
		return nil, err
	}

	if response.SecretString == nil {
		return nil, nil
	}

	return confmap.NewRetrieved(*response.SecretString)
}

func (*provider) Scheme() string {
	return schemeName
}

func (*provider) Shutdown(context.Context) error {
	return nil
}
