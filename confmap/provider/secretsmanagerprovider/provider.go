// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package secretsmanagerprovider // import "github.com/open-telemetry/opentelemetry-collector-contrib/confmap/provider/secretsmanagerprovider"

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"go.opentelemetry.io/collector/confmap"
)

type secretsManagerClient interface {
	GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
}

const (
	schemeName = "secretsmanager"
)

type provider struct {
	client secretsManagerClient
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
	return &provider{client: nil}
}

func (provider *provider) Retrieve(ctx context.Context, uri string, _ confmap.WatcherFunc) (*confmap.Retrieved, error) {
	if !strings.HasPrefix(uri, schemeName+":") {
		return nil, fmt.Errorf("%q uri is not supported by %q provider", uri, schemeName)
	}

	// initialize the secrets manager client in the first call of Retrieve
	if provider.client == nil {
		cfg, err := config.LoadDefaultConfig(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to load configurations to initialize an AWS SDK client, error: %w", err)
		}

		provider.client = secretsmanager.NewFromConfig(cfg)
	}

	// Remove schemeName and split by # to get the json key
	secretArn, secretJSONKey, jsonKeyFound := strings.Cut(strings.Replace(uri, schemeName+":", "", 1), "#")

	input := &secretsmanager.GetSecretValueInput{
		SecretId: &secretArn,
	}

	response, err := provider.client.GetSecretValue(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("error gtting secret: %w", err)
	}

	if response.SecretString == nil {
		return nil, nil
	}

	if jsonKeyFound {
		var secretFieldsMap map[string]any
		err := json.Unmarshal([]byte(*response.SecretString), &secretFieldsMap)
		if err != nil {
			return nil, fmt.Errorf("error unmarshalling secret string: %w", err)
		}

		secretValue, ok := secretFieldsMap[secretJSONKey]
		if !ok {
			return nil, fmt.Errorf("field %q not found in secret map", secretJSONKey)
		}

		return confmap.NewRetrieved(secretValue)
	}

	return confmap.NewRetrieved(*response.SecretString)
}

func (*provider) Scheme() string {
	return schemeName
}

func (*provider) Shutdown(context.Context) error {
	return nil
}
