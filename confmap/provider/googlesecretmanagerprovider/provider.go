// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate make mdatagen

package googlesecretmanagerprovider // import "github.com/open-telemetry/opentelemetry-collector-contrib/confmap/provider/googlesecretmanagerprovider"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	gax "github.com/googleapis/gax-go/v2"
	"github.com/googleapis/gax-go/v2/apierror"
	"go.opentelemetry.io/collector/confmap"
	"go.uber.org/zap"
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
	logger *zap.Logger
}

func NewFactory() confmap.ProviderFactory {
	return confmap.NewProviderFactory(newProvider)
}

func newProvider(ps confmap.ProviderSettings) confmap.Provider {
	return &provider{client: nil, logger: ps.Logger}
}

// Retrieve fetches a secret from Google Secret Manager.
//
// The URI format is:
//
//	googlesecretmanager:projects/<project>/secrets/<secret>/versions/<version>[#<json-key>][:-<default>]
//
// Optional suffixes (modelled after the AWS secretsmanagerprovider):
//   - "#<json-key>"  — parse the secret value as JSON and return the value of
//     the given key.
//   - ":-<default>"  — return <default> when the secret name is empty or the
//     requested JSON key is absent from the secret payload.
func (p *provider) Retrieve(ctx context.Context, uri string, _ confmap.WatcherFunc) (*confmap.Retrieved, error) {
	if !strings.HasPrefix(uri, schemeName+":") {
		return nil, fmt.Errorf("%q: %w", uri, ErrURINotSupported)
	}

	spec := strings.TrimPrefix(uri, schemeName+":")

	// Split by :- to extract an optional default value.
	selector, defaultValue, hasDefaultValue := strings.Cut(spec, ":-")

	// Split by # to extract an optional JSON key.
	secretName, secretJSONKey, jsonKeyFound := strings.Cut(selector, "#")

	// If the secret name is empty but a default was provided, return it directly.
	if secretName == "" && hasDefaultValue {
		p.logger.Warn("Google Secret Manager selector is empty, falling back to default value")
		return confmap.NewRetrieved(defaultValue)
	}

	// Lazy-initialise the Google Secret Manager client on first use.
	if p.client == nil {
		client, err := secretmanager.NewClient(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to create a Google secret manager client: %w", err)
		}
		p.client = client
	}

	resp, err := p.client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
		Name: secretName,
	})
	if err != nil {
		apiErr, ok := apierror.FromError(err)
		if !ok {
			return nil, fmt.Errorf("%w : %w", ErrAccessSecretVersion, err)
		}
		return nil, fmt.Errorf("%w: %v", ErrAccessSecretVersion, apiErr.Error())
	}

	secretString := string(resp.GetPayload().GetData())

	// If a JSON key was requested, parse the secret payload as a JSON object
	// and return the value of that key.
	if jsonKeyFound {
		var secretFieldsMap map[string]any
		if err := json.Unmarshal([]byte(secretString), &secretFieldsMap); err != nil {
			return nil, fmt.Errorf("error unmarshalling secret string as JSON: %w", err)
		}

		secretValue, ok := secretFieldsMap[secretJSONKey]
		if !ok {
			if hasDefaultValue {
				p.logger.Warn("field not found in secret JSON map, falling back to default value",
					zap.String("field", secretJSONKey))
				return confmap.NewRetrieved(defaultValue)
			}
			return nil, fmt.Errorf("field %q not found in secret JSON map", secretJSONKey)
		}

		return confmap.NewRetrieved(secretValue)
	}

	return confmap.NewRetrieved(secretString)
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
