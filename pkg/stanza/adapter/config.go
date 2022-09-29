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

	"go.opentelemetry.io/collector/config"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

// BaseConfig is the common configuration of a stanza-based receiver
type BaseConfig struct {
	config.ReceiverSettings `mapstructure:",squash"`
	Operators               []operator.Config   `mapstructure:"operators"`
	Converter               ConverterConfig     `mapstructure:"converter"`
	StorageID               *config.ComponentID `mapstructure:"storage"`
}

// ConverterConfig controls how the internal entry.Entry to plog.Logs converter
// works.
type ConverterConfig struct {
	// MaxFlushCount defines the maximum number of entries that can be
	// accumulated before flushing them for further processing.
	MaxFlushCount uint `mapstructure:"max_flush_count"`
	// FlushInterval defines how often to flush the converted and accumulated
	// log entries.
	FlushInterval time.Duration `mapstructure:"flush_interval"`
	// WorkerCount defines how many worker goroutines used for entry.Entry to
	// log records translation should be spawned.
	// By default: math.Max(1, runtime.NumCPU()/4) workers are spawned.
	WorkerCount int `mapstructure:"worker_count"`
}
