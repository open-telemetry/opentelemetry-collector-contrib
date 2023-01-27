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
	"fmt"

	"github.com/Azure/azure-amqp-common-go/v4/conn"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
)

type logFormat string

const (
	defaultLogFormat logFormat = ""
	rawLogFormat     logFormat = "raw"
	azureLogFormat   logFormat = "azure"
)

var (
	validFormats         = []logFormat{defaultLogFormat, rawLogFormat, azureLogFormat}
	errMissingConnection = errors.New("missing connection")
)

type Config struct {
	Connection string        `mapstructure:"connection"`
	Partition  string        `mapstructure:"partition"`
	Offset     string        `mapstructure:"offset"`
	StorageID  *component.ID `mapstructure:"storage"`
	Format     string        `mapstructure:"format"`
}

func isValidFormat(format string) bool {
	for _, validFormat := range validFormats {
		if logFormat(format) == validFormat {
			return true
		}
	}
	return false
}

// Validate config
func (config *Config) Validate() error {
	if config.Connection == "" {
		return errMissingConnection
	}
	if _, err := conn.ParsedConnectionFromStr(config.Connection); err != nil {
		return err
	}
	if !isValidFormat(config.Format) {
		return fmt.Errorf("invalid format; must be one of %#v", validFormats)
	}
	return nil
}

func (config *Config) Unmarshal(parser *confmap.Conf) error {
	if parser == nil {
		return nil
	}
	err := parser.Unmarshal(config) // , confmap.WithErrorUnused()) // , cmpopts.IgnoreUnexported(metadata.MetricSettings{}))
	if err != nil {
		return err
	}
	return nil
}
