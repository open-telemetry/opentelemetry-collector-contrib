// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension"

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes/source"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/httpserver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/metadata"
	datadogconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/hostmetadata"
)

type factory struct {
	onceProvider   sync.Once
	sourceProvider source.Provider
	providerErr    error
}

func (f *factory) SourceProvider(set component.TelemetrySettings, configHostname string, timeout time.Duration) (source.Provider, error) {
	f.onceProvider.Do(func() {
		f.sourceProvider, f.providerErr = hostmetadata.GetSourceProvider(set, configHostname, timeout)
	})
	return f.sourceProvider, f.providerErr
}

// NewFactory creates a factory for the Datadog extension.
func NewFactory() extension.Factory {
	f := &factory{}
	return extension.NewFactory(
		metadata.Type,
		f.createDefaultConfig,
		f.create,
		metadata.ExtensionStability,
	)
}

func (f *factory) createDefaultConfig() component.Config {
	return &Config{
		ClientConfig: confighttp.NewDefaultClientConfig(),
		API: datadogconfig.APIConfig{
			Site:             datadogconfig.DefaultSite,
			FailOnInvalidKey: true,
		},
		HTTPConfig: &httpserver.Config{
			ServerConfig: confighttp.ServerConfig{
				Endpoint: httpserver.DefaultServerEndpoint,
			},
			Path: "/metadata",
		},
	}
}

func (f *factory) create(ctx context.Context, set extension.Settings, cfg component.Config) (extension.Extension, error) {
	extensionConfig, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config type: %T", cfg)
	}
	// set timeout to 25 seconds to avoid default kube liveness probe of 10 seconds * 3 attempts
	hostProvider, err := f.SourceProvider(set.TelemetrySettings, extensionConfig.Hostname, time.Second*25)
	if err != nil {
		return nil, err
	}
	// Create the real UUID provider for the extension
	uuidProvider := &realUUIDProvider{}

	return newExtension(ctx, extensionConfig, set, hostProvider, uuidProvider)
}
