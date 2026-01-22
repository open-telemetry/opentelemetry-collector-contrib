// Copyright SAP Cloud Infrastructure
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package auditdreceiver // import "github.com/cloudoperators/opentelemetry-collector-contrib/receiver/auditdreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

var Type = component.MustNewType("auditd")

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		Type,
		createDefaultConfig,
		receiver.WithLogs(createLogsReceiver, component.StabilityLevelAlpha))
}

func createLogsReceiver(_ context.Context, settings receiver.Settings, baseCfg component.Config, consumer consumer.Logs) (receiver.Logs, error) {
	cfg := baseCfg.(*AuditdReceiverConfig)
	return newAuditd(cfg, consumer, settings)
}
