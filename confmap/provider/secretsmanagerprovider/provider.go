// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package secretsmanagerprovider // import "github.com/open-telemetry/opentelemetry-collector-contrib/confmap/provider/secretsmanagerprovider"

import (
	"context"
	"fmt"
	"strings"

	"encoding/json"

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

// New returns a new confmap.Provider that reads the configuration from the given AWS Secrets Manager Name or ARN.
//
// This Provider supports "secretsmanager" scheme, and can be called with a selector:
// `secretsmanager:NAME_OR_ARN`
func New() confmap.Provider {
	return &provider{client: nil}
}

func (provider *provider) Retrieve(ctx context.Context, uri string, _ confmap.WatcherFunc) (*confmap.Retrieved, error) {
	if !strings.HasPrefix(uri, schemeName+":") {
		return nil, fmt.Errorf("%q uri is not supported by %q provider", uri, schemeName)
	}

	// initialize the secrets manager client in the first call of Retrieve
	if provider.client == nil {
	    cfg, err := config.LoadDefaultConfig(context.Background())

	    if err != nil {
	        return nil, fmt.Errorf("failed to load configurations to initialize an AWS SDK client, error: %w", err)
	    }

	    provider.client = secretsmanager.NewFromConfig(cfg)
    }

	secretArn := strings.Replace(uri, schemeName+":", "", 1)

	input := &secretsmanager.GetSecretValueInput{
		SecretId: &secretArn,
	}

    // Split string by # to get the json key for secret
	var splits = strings.SplitN(secretArn, "#", 2)
	secretArn = splits[0]

	response, err := provider.client.GetSecretValue(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("error gtting secret: %w", err)
	}

	if response.SecretString == nil {
		return nil, nil
	}

	if len(splits) == 1 {
	    return confmap.NewRetrieved(*response.SecretString)
	} else {
	    var secretMap map[string]interface{}
        err := json.Unmarshal([]byte(*response.SecretString), &secretMap)
        if err != nil {
            return nil, fmt.Errorf("error unmarshalling secret string: %w", err)
        }

        key := splits[1]
        value, ok := secretMap[key]
        if !ok {
            return nil, fmt.Errorf("key %s not found in secret map", key)
        }

        return confmap.NewRetrieved(value)
    }
}

func (*provider) Scheme() string {
	return schemeName
}

func (*provider) Shutdown(context.Context) error {
	return nil
}
