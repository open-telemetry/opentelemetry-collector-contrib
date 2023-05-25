// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filestorage // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/filestorage"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
)

// The value of extension "type" in configuration.
const typeStr component.Type = "file_storage"

const (
	// use default bbolt value
	// https://github.com/etcd-io/bbolt/blob/d5db64bdbfdee1cb410894605f42ffef898f395d/cmd/bbolt/main.go#L1955
	defaultMaxTransactionSize         = 65536
	defaultReboundTriggerThresholdMib = 10
	defaultReboundNeededThresholdMib  = 100
	defaultCompactionInterval         = time.Second * 5
)

// NewFactory creates a factory for HostObserver extension.
func NewFactory() extension.Factory {
	return extension.NewFactory(
		typeStr,
		createDefaultConfig,
		createExtension,
		component.StabilityLevelBeta,
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		Directory: getDefaultDirectory(),
		Compaction: &CompactionConfig{
			Directory:                  getDefaultDirectory(),
			OnStart:                    false,
			OnRebound:                  false,
			MaxTransactionSize:         defaultMaxTransactionSize,
			ReboundNeededThresholdMiB:  defaultReboundNeededThresholdMib,
			ReboundTriggerThresholdMiB: defaultReboundTriggerThresholdMib,
			CheckInterval:              defaultCompactionInterval,
		},
		Timeout: time.Second,
	}
}

func createExtension(
	_ context.Context,
	params extension.CreateSettings,
	cfg component.Config,
) (extension.Extension, error) {
	return newLocalFileStorage(params.Logger, cfg.(*Config))
}
