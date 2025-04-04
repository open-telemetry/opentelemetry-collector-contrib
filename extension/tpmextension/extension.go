// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tpmextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/tpmextension"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
)

type tpmExtension struct {
	config            *Config
	cancel            context.CancelFunc
	telemetrySettings component.TelemetrySettings
}

var _ extension.Extension = (*tpmExtension)(nil)

func newTPMExtension(extensionCfg *Config, settings extension.Settings) (extension.Extension, error) {
	settingsExtension := &tpmExtension{
		config:            extensionCfg,
		telemetrySettings: settings.TelemetrySettings,
	}
	return settingsExtension, nil
}

func (extension *tpmExtension) Start(_ context.Context, _ component.Host) error {
	extension.telemetrySettings.Logger.Info("starting up tpm extension")

	return nil
}

func (extension *tpmExtension) Shutdown(_ context.Context) error {
	extension.telemetrySettings.Logger.Info("shutting down tmp extension")
	if extension.cancel != nil {
		extension.cancel()
	}
	return nil
}
