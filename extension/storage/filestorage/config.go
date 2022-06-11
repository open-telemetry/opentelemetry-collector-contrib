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
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"

	"go.opentelemetry.io/collector/config"
)

// Config defines configuration for http forwarder extension.
type Config struct {
	config.ExtensionSettings `mapstructure:",squash"`

	Directory string        `mapstructure:"directory,omitempty"`
	Timeout   time.Duration `mapstructure:"timeout,omitempty"`

	Compaction *CompactionConfig `mapstructure:"compaction,omitempty"`
}

type CompactionConfig struct {
	OnStart            bool   `mapstructure:"on_start,omitempty"`
	Directory          string `mapstructure:"directory,omitempty"`
	MaxTransactionSize int64  `mapstructure:"max_transaction_size,omitempty"`
}

func (cfg *Config) Validate() error {
	var dirs []string
	if cfg.Compaction.OnStart {
		dirs = []string{cfg.Directory, cfg.Compaction.Directory}
	} else {
		dirs = []string{cfg.Directory}
	}
	for _, dir := range dirs {
		info, err := os.Stat(dir)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("directory must exist: %v", err)
			}
			if fsErr, ok := err.(*fs.PathError); ok {
				return fmt.Errorf(
					"problem accessing configured directory: %s, err: %v",
					dir, fsErr,
				)
			}

		}
		if !info.IsDir() {
			return fmt.Errorf("%s is not a directory", dir)
		}
	}

	if cfg.Compaction.MaxTransactionSize < 0 {
		return errors.New("max transaction size for compaction cannot be less than 0")
	}

	return nil
}
