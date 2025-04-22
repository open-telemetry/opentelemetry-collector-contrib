// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package secretsmanagerprovider // import "github.com/open-telemetry/opentelemetry-collector-contrib/confmap/provider/secretsmanagerprovider"

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"go.opentelemetry.io/collector/confmap"
	"go.uber.org/zap"
)

type secretsManagerClient interface {
	GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
}

const (
	schemeName = "secretsmanager"
)

type provider struct {
	client secretsManagerClient
	logger *zap.Logger
}

// NewFactory returns a new confmap.ProviderFactory that creates a confmap.Provider
// which reads configuration the given AWS Secrets Manager Name or ARN.
//
// This Provider supports "secretsmanager" scheme, and can be called with a selector:
// `secretsmanager:NAME_OR_ARN`
//
// A default value for unset variable can be provided after :- suffix, for example:
// `secretsmanager:NAME_OR_ARN:-default_value`
func NewFactory() confmap.ProviderFactory {
	return confmap.NewProviderFactory(newWithSettings)
}

func newWithSettings(ps confmap.ProviderSettings) confmap.Provider {
	return &provider{client: nil, logger: ps.Logger}
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

	spec := strings.TrimPrefix(uri, schemeName+":")
	// split by :- to get the default value
	selector, defaultValue, hasDefaultValue := strings.Cut(spec, ":-")
	// split by # to get the json key
	secretArn, secretJSONKey, jsonKeyFound := strings.Cut(selector, "#")

	if secretArn == "" && hasDefaultValue {
		provider.logger.Warn("secret manager selector empty, falling back to default value")
		return confmap.NewRetrieved(defaultValue)
	}

	input := &secretsmanager.GetSecretValueInput{
		SecretId: &secretArn,
	}

	response, err := provider.client.GetSecretValue(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("error getting secret: %w", err)
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
			if hasDefaultValue {
				provider.logger.Warn("field not found in secret map, falling back to default value")
				return confmap.NewRetrieved(defaultValue)
			}
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
