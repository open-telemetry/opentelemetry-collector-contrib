// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package parameterstoreprovider // import "github.com/open-telemetry/opentelemetry-collector-contrib/confmap/provider/parameterstoreprovider"

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"go.opentelemetry.io/collector/confmap"
)

type ssmClient interface {
	GetParameter(ctx context.Context, params *ssm.GetParameterInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error)
}

const (
	schemeName = "parameterstore"
)

type provider struct {
	client ssmClient
}

// NewFactory returns a new confmap.ProviderFactory that creates a confmap.Provider
// which reads configuration using the given AWS SSM ParameterStore Name or ARN.
//
// This Provider supports "parameterstore" scheme, and can be called with a selector:
// `parameterstore:NAME_OR_ARN`
func NewFactory() confmap.ProviderFactory {
	return confmap.NewProviderFactory(newWithSettings)
}

func newWithSettings(_ confmap.ProviderSettings) confmap.Provider {
	return &provider{client: nil}
}

func (provider *provider) Retrieve(ctx context.Context, rawURI string, _ confmap.WatcherFunc) (*confmap.Retrieved, error) {
	uri, err := url.Parse(rawURI)
	if err != nil {
		return nil, fmt.Errorf("failed to parse uri %q: %w", rawURI, err)
	}

	if uri.Scheme != schemeName {
		return nil, fmt.Errorf("%q uri is not supported by %q provider", rawURI, schemeName)
	}

	if err = provider.ensureClient(ctx); err != nil {
		return nil, err
	}

	// extract relevant query and fragment values
	jsonField := uri.EscapedFragment()
	withDecryption := uri.Query().Get("withDecryption") == "true"

	// reset scheme/query/fragment
	uri.Scheme = ""
	uri.RawQuery = ""
	uri.Fragment = ""
	uri.RawFragment = ""

	parameterName := uri.String()

	req := &ssm.GetParameterInput{
		Name:           aws.String(parameterName),
		WithDecryption: aws.Bool(withDecryption),
	}

	response, err := provider.client.GetParameter(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("error getting parameter: %w", err)
	}

	if jsonField != "" {
		return provider.retrieveJSONField(*response.Parameter.Value, jsonField)
	}

	return confmap.NewRetrieved(*response.Parameter.Value)
}

func (provider *provider) ensureClient(ctx context.Context) error {
	// initialize the ssm client in the first call of Retrieve
	if provider.client == nil {
		cfg, err := config.LoadDefaultConfig(ctx)

		if err != nil {
			return fmt.Errorf("failed to load configurations to initialize an AWS SDK client, error: %w", err)
		}

		provider.client = ssm.NewFromConfig(cfg)
	}

	return nil
}

func (*provider) retrieveJSONField(rawJSON, field string) (*confmap.Retrieved, error) {
	var fieldsMap map[string]any
	err := json.Unmarshal([]byte(rawJSON), &fieldsMap)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling parameter string: %w", err)
	}

	fieldValue, ok := fieldsMap[field]
	if !ok {
		return nil, fmt.Errorf("field %q not found in fields map", field)
	}

	return confmap.NewRetrieved(fieldValue)
}

func (*provider) Scheme() string {
	return schemeName
}

func (*provider) Shutdown(context.Context) error {
	return nil
}
