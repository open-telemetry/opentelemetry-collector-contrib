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
	"bufio"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

const operatorType = "file_input"

func init() {
	operator.Register(operatorType, func() operator.Builder { return NewConfig() })
}

// NewConfig creates a new input config with default values
func NewConfig() *Config {
	return NewConfigWithID(operatorType)
}

// NewConfigWithID creates a new input config with default values
func NewConfigWithID(operatorID string) *Config {
	return &Config{
		InputConfig:     helper.NewInputConfig(operatorID, operatorType),
		Config:          *fileconsumer.NewConfig(),
		MultilineConfig: helper.NewMultilineConfig(),
	}
}

// Config is the configuration of a file input operator
type Config struct {
	helper.InputConfig     `mapstructure:",squash"`
	fileconsumer.Config    `mapstructure:",squash"`
	helper.MultilineConfig `mapstructure:"multiline,omitempty"`
}

// Build will build a file input operator from the supplied configuration
func (c Config) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	inputOperator, err := c.InputConfig.Build(logger)
	if err != nil {
		return nil, err
	}

	var preEmitOptions []preEmitOption
	if c.IncludeFileName {
		preEmitOptions = append(preEmitOptions, setFileName)
	}
	if c.IncludeFilePath {
		preEmitOptions = append(preEmitOptions, setFilePath)
	}
	if c.IncludeFileNameResolved {
		preEmitOptions = append(preEmitOptions, setFileNameResolved)
	}
	if c.IncludeFilePathResolved {
		preEmitOptions = append(preEmitOptions, setFilePathResolved)
	}

	var toBody toBodyFunc = func(token []byte) interface{} {
		return string(token)
	}
	if helper.IsNop(c.Config.EncodingConfig.Encoding) {
		toBody = func(token []byte) interface{} {
			return token
		}
	}

	input := &Input{
		InputOperator:  inputOperator,
		toBody:         toBody,
		preEmitOptions: preEmitOptions,
	}
	var splitter bufio.SplitFunc
	if c.LineStartPattern != "" || c.LineEndPattern != "" {
		splitter, err = c.buildMultilineSplitter()
		if err != nil {
			return nil, err
		}
	}
	input.fileConsumer, err = c.Config.Build(logger, input.emit, fileconsumer.WithCustomizedSplitter(splitter))
	if err != nil {
		return nil, err
	}

	return input, nil
}

func (c Config) buildMultilineSplitter() (bufio.SplitFunc, error) {
	enc, err := c.EncodingConfig.Build()
	if err != nil {
		return nil, err
	}
	flusher := c.Flusher.Build()
	splitter, err := c.MultilineConfig.Build(enc.Encoding, false, flusher, int(c.MaxLogSize))
	if err != nil {
		return nil, err
	}
	return splitter, nil
}
