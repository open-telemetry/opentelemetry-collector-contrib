// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filetelemetrypolicyextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/filetelemetrypolicyextension"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
)

var _ extension.Extension = (*fileTelemetryPolicyExtension)(nil)

type fileTelemetryPolicyExtension struct {
	config   *Config
	settings extension.Settings
}

func newExtension(cfg *Config, settings extension.Settings) *fileTelemetryPolicyExtension {
	return &fileTelemetryPolicyExtension{
		config:   cfg,
		settings: settings,
	}
}

func (*fileTelemetryPolicyExtension) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (*fileTelemetryPolicyExtension) Shutdown(_ context.Context) error {
	return nil
}
