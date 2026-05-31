// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package activedirectoryinvreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/activedirectoryinvreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/activedirectoryinvreceiver/internal/metadata"
)

// NewFactory creates a factory for Active Directory Inventory receiver
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithLogs(createLogsReceiver, metadata.LogsStability),
	)
}

// createDefaultConfig creates the default configuration for the receiver
func createDefaultConfig() component.Config {
	return &ADConfig{
		BaseDN:       "",
		Attributes:   []string{"name", "mail", "department", "manager", "memberOf"},
		PollInterval: 24 * time.Hour,
	}
}

func createLogsReceiver(
	_ context.Context,
	params receiver.Settings,
	rConf component.Config,
	consumer consumer.Logs,
) (receiver.Logs, error) {
	cfg := rConf.(*ADConfig)
	adsiClient := &adsiClient{}
	adRuntime := &adRuntimeInfo{}
	rcvr := newLogsReceiver(cfg, params.Logger, adsiClient, adRuntime, consumer)
	return rcvr, nil
}
