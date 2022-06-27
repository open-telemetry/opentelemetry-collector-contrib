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

package file // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/file"

import (
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/internal/fileconsumer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

func init() {
	operator.Register("file_input", func() operator.Builder { return NewConfig("") })
}

// NewConfig creates a new input config with default values
func NewConfig(operatorID string) *Config {
	return &Config{
		InputConfig: helper.NewInputConfig(operatorID, "file_input"),
		Config:      *fileconsumer.NewConfig(),
	}
}

// Config is the configuration of a file input operator
type Config struct {
	helper.InputConfig  `mapstructure:",squash" yaml:",inline"`
	fileconsumer.Config `mapstructure:",squash" yaml:",inline"`
}

// Build will build a file input operator from the supplied configuration
func (c Config) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	inputOperator, err := c.InputConfig.Build(logger)
	if err != nil {
		return nil, err
	}

	fileNameField := entry.NewNilField()
	if c.IncludeFileName {
		fileNameField = entry.NewAttributeField("log.file.name")
	}

	filePathField := entry.NewNilField()
	if c.IncludeFilePath {
		filePathField = entry.NewAttributeField("log.file.path")
	}

	fileNameResolvedField := entry.NewNilField()
	if c.IncludeFileNameResolved {
		fileNameResolvedField = entry.NewAttributeField("log.file.name_resolved")
	}

	filePathResolvedField := entry.NewNilField()
	if c.IncludeFilePathResolved {
		filePathResolvedField = entry.NewAttributeField("log.file.path_resolved")
	}

	var toBody toBodyFunc = func(token []byte) interface{} {
		return string(token)
	}
	if helper.IsNop(c.Config.Splitter.EncodingConfig.Encoding) {
		toBody = func(token []byte) interface{} {
			return token
		}
	}

	input := &Input{
		InputOperator:         inputOperator,
		FilePathField:         filePathField,
		FileNameField:         fileNameField,
		FilePathResolvedField: filePathResolvedField,
		FileNameResolvedField: fileNameResolvedField,
		toBody:                toBody,
	}

	input.fileConsumer, err = c.Config.Build(logger, input.emit)
	if err != nil {
		return nil, err
	}

	return input, nil
}
