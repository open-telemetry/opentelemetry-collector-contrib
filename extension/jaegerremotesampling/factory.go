// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jaegerremotesampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/jaegerremotesampling"

import (
	"context"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/featuregate"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/jaegerremotesampling/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/jaegerremotesampling/jaegerremotesamplingdeprecated"
)

// NewFactory creates a factory for the jaeger remote sampling extension.
func NewFactory() extension.Factory {
	if !protoGate.IsEnabled() {
		return jaegerremotesamplingdeprecated.NewFactory()
	}

	return extension.NewFactory(
		metadata.Type,
		createDefaultConfig,
		createExtension,
		metadata.ExtensionStability,
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		HTTPServerSettings: &confighttp.HTTPServerSettings{
			Endpoint: ":5778",
		},
		GRPCServerSettings: &configgrpc.GRPCServerSettings{
			NetAddr: confignet.NetAddr{
				Endpoint:  ":14250",
				Transport: "tcp",
			},
		},
		Source: Source{},
	}
}

var once sync.Once

func logDeprecation(logger *zap.Logger) {
	once.Do(func() {
		logger.Warn("jaegerremotesampling extension will deprecate Thrift-gen and replace it with Proto-gen to be compatbible to jaeger 1.42.0 and higher. See https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/18485 for more details.")
	})
}

const protoInsteadOfThrift = "extension.jaegerremotesampling.replaceThriftWithProto"

var protoGate = featuregate.GlobalRegistry().MustRegister(
	protoInsteadOfThrift,
	featuregate.StageBeta,
	featuregate.WithRegisterDescription(
		"When enabled, the jaegerremotesampling will use Proto-gen over Thrift-gen.",
	),
)

func createExtension(_ context.Context, set extension.CreateSettings, cfg component.Config) (extension.Extension, error) {
	return newExtension(cfg.(*Config), set.TelemetrySettings), nil
}
