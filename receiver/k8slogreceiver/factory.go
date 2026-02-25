// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8slogreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8slogreceiver"
import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8slogreceiver/internal/metadata"
)

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithLogs(
			createLogsReceiver,
			metadata.LogsStability,
		),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		Discovery: SourceConfig{
			Mode:        DefaultMode,
			HostRoot:    DefaultHostRoot,
			NodeFromEnv: DefaultNodeFromEnv,
			K8sAPI:      k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount},
			RuntimeAPIs: []RuntimeAPIConfig{
				{
					&criConfig{
						baseRuntimeAPIConfig: baseRuntimeAPIConfig{
							Type: "cri",
						},
					},
				},
			},
		},
	}
}

func createLogsReceiver(
	_ context.Context,
	settings receiver.Settings,
	cfg component.Config,
	consumer consumer.Logs,
) (receiver.Logs, error) {
	settings.Logger.Error("k8slogreceiver is not yet implemented")
	rCfg, ok := cfg.(*Config)
	if !ok {
		return nil, errors.New("failed to cast config to k8slogreceiver.Config")
	}
	return newReceiver(settings, rCfg, consumer)
}
