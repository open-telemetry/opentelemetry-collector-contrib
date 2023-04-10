// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
