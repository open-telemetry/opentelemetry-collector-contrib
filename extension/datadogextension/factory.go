// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !aix && !solaris

package datadogextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension"

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/attributes/source"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/httpserver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/agentcomponents"
	datadogconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/hostmetadata"
)

type factory struct {
	onceProvider   sync.Once
	sourceProvider source.Provider
	providerErr    error

	// Host-supplied, appended to the extension's own. See WithConfigOptions.
	configOptions []agentcomponents.ConfigOption
}

// Option configures the Datadog extension factory.
type Option func(*factory)

// WithConfigOptions appends options to those the extension uses to build its internal
// Agent config component, letting an embedding host keep that config in sync with its
// own — for example to propagate an API key rotation.
//
// They are applied after the extension's own, so a value they set wins. They run before
// the config component's schema is built, so they may Set values but not read them.
func WithConfigOptions(o ...agentcomponents.ConfigOption) Option {
	return func(f *factory) {
		f.configOptions = append(f.configOptions, o...)
	}
}

func (f *factory) SourceProvider(set component.TelemetrySettings, configHostname string, timeout time.Duration) (source.Provider, error) {
	f.onceProvider.Do(func() {
		f.sourceProvider, f.providerErr = hostmetadata.GetSourceProvider(set, configHostname, timeout)
	})
	return f.sourceProvider, f.providerErr
}

// NewFactory creates a factory for the Datadog extension.
func NewFactory() extension.Factory {
	return NewFactoryWithOptions()
}

// NewFactoryWithOptions creates a factory for the Datadog extension. With no options it
// is equivalent to NewFactory.
func NewFactoryWithOptions(opts ...Option) extension.Factory {
	f := &factory{}
	for _, opt := range opts {
		opt(f)
	}
	return extension.NewFactory(
		metadata.Type,
		f.createDefaultConfig,
		f.create,
		metadata.ExtensionStability,
	)
}

func (*factory) createDefaultConfig() component.Config {
	netAddr := confignet.NewDefaultAddrConfig()
	netAddr.Transport = confignet.TransportTypeTCP
	netAddr.Endpoint = httpserver.DefaultServerEndpoint
	serverConfig := confighttp.NewDefaultServerConfig()
	serverConfig.NetAddr = netAddr
	return &Config{
		ClientConfig: confighttp.NewDefaultClientConfig(),
		API: datadogconfig.APIConfig{
			Site:             datadogconfig.DefaultSite,
			FailOnInvalidKey: true,
		},
		HTTPConfig: &httpserver.Config{
			ServerConfig: serverConfig,
			Path:         "/metadata",
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

	return newExtension(ctx, extensionConfig, set, hostProvider, uuidProvider, f.configOptions...)
}
