// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ssmprovider // import "github.com/open-telemetry/opentelemetry-collector-contrib/confmap/provider/ssmprovider"

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"go.opentelemetry.io/collector/confmap"
)

type ssmClient interface {
	GetParameter(ctx context.Context, params *ssm.GetParameterInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error)
}

const (
	schemeName = "ssm"
)

type provider struct {
	client ssmClient
}

func NewFactory() confmap.ProviderFactory {
	return confmap.NewProviderFactory(newWithSettings)
}

func newWithSettings(_ confmap.ProviderSettings) confmap.Provider {
	return &provider{client: nil}
}

func (provider *provider) Retrieve(ctx context.Context, uri string, _ confmap.WatcherFunc) (*confmap.Retrieved, error) {
	if !strings.HasPrefix(uri, schemeName+":") {
		return nil, fmt.Errorf("%q uri is not supported by %q provider", uri, schemeName)
	}

	if provider.client == nil {
		cfg, err := config.LoadDefaultConfig(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to load configurations to initialize AWS SDK client, error: %w", err)
		}
		provider.client = ssm.NewFromConfig(cfg)
	}

	paramPath, jsonKey, hasJsonKey := strings.Cut(strings.TrimPrefix(uri, schemeName+":"), "#")

	input := &ssm.GetParameterInput{
		Name:           &paramPath,
		WithDecryption: true,
	}

	response, err := provider.client.GetParameter(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("error getting parameter: %w", err)
	}

	if response.Parameter == nil || response.Parameter.Value == nil {
		return nil, fmt.Errorf("parameter %q not found or has no value", paramPath)
	}

	paramValue := *response.Parameter.Value

	if hasJsonKey {
		var jsonData map[string]interface{}
		err := json.Unmarshal([]byte(paramValue), &jsonData)
		if err != nil {
			return nil, fmt.Errorf("error parsing JSON from parameter: %w", err)
		}

		value, ok := jsonData[jsonKey]
		if !ok {
			return nil, fmt.Errorf("key %q not found in JSON parameter", jsonKey)
		}

		return confmap.NewRetrieved(value)
	}

	return confmap.NewRetrieved(paramValue)
}

func (*provider) Scheme() string {
	return schemeName
}

func (*provider) Shutdown(context.Context) error {
	return nil
}
