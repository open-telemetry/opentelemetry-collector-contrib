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
}

func (cfg *Config) Validate() error {
	info, err := os.Stat(cfg.Directory)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("directory must exist: %v", err)
		}
		if fsErr, ok := err.(*fs.PathError); ok {
			return fmt.Errorf(
				"problem accessing configured directory: %s, err: %v",
				cfg.Directory, fsErr,
			)
		}

	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", cfg.Directory)
	}

	return nil
}
