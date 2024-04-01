// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package file // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/file"

import (
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/decode"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

var operatorType = component.MustNewType("file_input")

func init() {
	operator.RegisterFactory(NewFactory())
}

// NewFactory creates a new factory.
func NewFactory() operator.Factory {
	return operator.NewFactory(operatorType, newDefaultConfig, createOperator)
}

func newDefaultConfig(operatorID string) component.Config {
	return &Config{
		InputConfig: helper.NewInputConfig(operatorID, operatorType.String()),
		Config:      *fileconsumer.NewConfig(),
	}
}

func createOperator(set component.TelemetrySettings, cfg component.Config) (operator.Operator, error) {
	c := cfg.(*Config)
	inputOperator, err := helper.NewInput(set, c.InputConfig)
	if err != nil {
		return nil, err
	}

	var toBody toBodyFunc = func(token []byte) any {
		return string(token)
	}
	if decode.IsNop(c.Config.Encoding) {
		toBody = func(token []byte) any {
			copied := make([]byte, len(token))
			copy(copied, token)
			return copied
		}
	}

	input := &Input{
		InputOperator: inputOperator,
		toBody:        toBody,
	}

	input.fileConsumer, err = c.Config.Build(set.Logger.Sugar(), input.emit)
	if err != nil {
		return nil, err
	}

	return input, nil
}
