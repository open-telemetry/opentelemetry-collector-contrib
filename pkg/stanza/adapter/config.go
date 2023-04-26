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

package adapter // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"

import (
	"time"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/consumerretry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

// BatchConfig is the configuration for logs batching. Logs are sent to the next consumer either when the batch size
// reaches MaxBatchSize or when the timeout is reached, whichever happens first.
type BatchConfig struct {
	// MaxBatchSize specifies the maximum number of logs to batch.
	MaxBatchSize uint `mapstructure:"max_batch_size"`

	// Timeout specifies the maximum duration for batching logs.
	Timeout time.Duration `mapstructure:"timeout"`
}

func NewDefaultBatchConfig() BatchConfig {
	return BatchConfig{
		MaxBatchSize: 100,
		Timeout:      100 * time.Millisecond,
	}
}

// BaseConfig is the common configuration of a stanza-based receiver
type BaseConfig struct {
	Operators      []operator.Config    `mapstructure:"operators"`
	StorageID      *component.ID        `mapstructure:"storage"`
	Batch          BatchConfig          `mapstructure:"batch"`
	RetryOnFailure consumerretry.Config `mapstructure:"retry_on_failure"`
}
