// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package solarwindsapmsettingsextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/solarwindsapmsettingsextension"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"
)

type solarwindsapmSettingsExtension struct {
	logger *zap.Logger
	config *Config
	cancel context.CancelFunc
}

func newSolarwindsApmSettingsExtension(extensionCfg *Config, logger *zap.Logger) (extension.Extension, error) {
	settingsExtension := &solarwindsapmSettingsExtension{
		config: extensionCfg,
		logger: logger,
	}
	return settingsExtension, nil
}

func (extension *solarwindsapmSettingsExtension) Start(_ context.Context, _ component.Host) error {
	extension.logger.Debug("Starting up solarwinds apm settings extension")
	_, extension.cancel = context.WithCancel(context.Background())
	return nil
}

func (extension *solarwindsapmSettingsExtension) Shutdown(_ context.Context) error {
	extension.logger.Debug("Shutting down solarwinds apm settings extension")
	return nil
}
