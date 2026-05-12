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

const (
	heartbeatIntervalNegotiationGateName = "extension.opampextension.HeartbeatIntervalNegotiation"
)

// HeartbeatIntervalNegotiationFeatureGate is the feature gate that controls the default value of reports_heartbeat.
var (
	HeartbeatIntervalNegotiationFeatureGate = featuregate.GlobalRegistry().MustRegister(
		heartbeatIntervalNegotiationGateName,
		featuregate.StageAlpha,
		featuregate.WithRegisterDescription("When enabled, the OpAMP extension defaults `reports_heartbeat` to true."),
		featuregate.WithRegisterFromVersion("v0.127.0"),
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
			ReportsHeartbeat:           HeartbeatIntervalNegotiationFeatureGate.IsEnabled(),
		},
		PPIDPollInterval: 5 * time.Second,
	}
}

func createExtension(_ context.Context, set extension.Settings, cfg component.Config) (extension.Extension, error) {
	return newOpampAgent(cfg.(*Config), set)
}
