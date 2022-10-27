// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"
import (
	"errors"

	"github.com/Azure/azure-amqp-common-go/v3/conn"
	"go.opentelemetry.io/collector/config"
)

var (
	errMissingConnection = errors.New("missing connection")
)

type Config struct {
	config.ReceiverSettings `mapstructure:",squash"`
	Connection              string              `mapstructure:"connection"`
	Partition               string              `mapstructure:"partition"`
	Offset                  string              `mapstructure:"offset"`
	StorageID               *config.ComponentID `mapstructure:"storage"`
}

// Validate config
func (config *Config) Validate() error {
	if config.Connection == "" {
		return errMissingConnection
	}
	if _, err := conn.ParsedConnectionFromStr(config.Connection); err != nil {
		return err
	}
	return nil
}
