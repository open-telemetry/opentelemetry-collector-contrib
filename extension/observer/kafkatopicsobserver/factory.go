// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkatopicsobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/kafkatopicsobserver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/kafkatopicsobserver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/configkafka"
)

const (
	defaultTopicsSyncInterval = 5 * time.Second
)

// NewFactory should be called to create a factory with default values.
func NewFactory() extension.Factory {
	return extension.NewFactory(
		metadata.Type,
		createDefaultConfig,
		createExtension,
		metadata.ExtensionStability,
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		ClientConfig:       configkafka.NewDefaultClientConfig(),
		TopicsSyncInterval: defaultTopicsSyncInterval,
	}
}

func createExtension(_ context.Context, settings extension.Settings, cfg component.Config) (extension.Extension, error) {
	settings.Logger.Warn("kafkatopicsobserver is deprecated; use kafkareceiver with topic regex support instead")
	return newObserver(settings.Logger, cfg.(*Config))
}
