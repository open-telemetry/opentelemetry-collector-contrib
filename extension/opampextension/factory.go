// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opampextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/opampextension"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/featuregate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/opampextension/internal/metadata"
)

const remoteRestartsFeatureGateName = "extension.opampextension.RemoteRestarts"

// RemoteRestartsFeatureGate is the feature gate that controls the remote restart capability for the OpAMP client.
var (
	RemoteRestartsFeatureGate = featuregate.GlobalRegistry().MustRegister(
		remoteRestartsFeatureGateName,
		featuregate.StageAlpha,
		featuregate.WithRegisterDescription("When enabled, the OpAMP extension supports restart commands from the OpAMP server through the `CommandType_Restart` command."),
		featuregate.WithRegisterFromVersion("v0.145.0"),
	)
)

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
		Server: &OpAMPServer{},
		Capabilities: Capabilities{
			ReportsEffectiveConfig:     true,
			ReportsHealth:              true,
			ReportsAvailableComponents: true,
			AcceptsRestartCommand:      false,
		},
		PPIDPollInterval: 5 * time.Second,
	}
}

func createExtension(_ context.Context, set extension.Settings, cfg component.Config) (extension.Extension, error) {
	return newOpampAgent(cfg.(*Config), set)
}
