// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package googlesecretsprovider // import "github.com/open-telemetry/opentelemetry-collector-contrib/confmap/provider/googlesecretsprovider"

import (
	"context"
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/confmap"
	"go.uber.org/zap"
)

type secretsManagerClient interface {
}

const (
	schemeName = "googlesecretsprovider"
)

type provider struct {
	client secretsManagerClient
	logger *zap.Logger
}

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

	return confmap.NewRetrieved("plaintext secret")
}

func (*provider) Scheme() string {
	return schemeName
}

func (*provider) Shutdown(context.Context) error {
	return nil
}
