// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package container // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/container"

import (
	"fmt"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/featuregate"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/errors"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/recombine"
)

const (
	operatorType                       = "container"
	recombineSourceIdentifier          = "log.file.path"
	recombineIsLastEntry               = "attributes.logtag == 'F'"
	removeOriginalTimeFieldFeatureFlag = "filelog.container.removeOriginalTimeField"
)

var removeOriginalTimeField = featuregate.GlobalRegistry().MustRegister(
	removeOriginalTimeFieldFeatureFlag,
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription("When enabled, deletes the original `time` field from the Log Attributes. Time is parsed to Timestamp field, which should be used instead."),
	featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/33389"),
)

func init() {
	operator.Register(operatorType, func() operator.Builder { return NewConfig() })
}

// NewConfig creates a new JSON parser config with default values
func NewConfig() *Config {
	return NewConfigWithID(operatorType)
}

// NewConfigWithID creates a new JSON parser config with default values
func NewConfigWithID(operatorID string) *Config {
	return &Config{
		ParserConfig:            helper.NewParserConfig(operatorID, operatorType),
		Format:                  "",
		AddMetadataFromFilePath: true,
		MaxLogSize:              0,
	}
}

// Config is the configuration of a Container parser operator.
type Config struct {
	helper.ParserConfig `mapstructure:",squash"`

	Format                  string          `mapstructure:"format"`
	AddMetadataFromFilePath bool            `mapstructure:"add_metadata_from_filepath"`
	MaxLogSize              helper.ByteSize `mapstructure:"max_log_size,omitempty"`
}

// Build will build a Container parser operator.
func (c Config) Build(set component.TelemetrySettings) (operator.Operator, error) {
	parserOperator, err := c.ParserConfig.Build(set)
	if err != nil {
		return nil, err
	}

	cLogEmitter := helper.NewLogEmitter(set)
	recombineParser, err := createRecombine(set, c, cLogEmitter)
	if err != nil {
		return nil, fmt.Errorf("failed to create internal recombine config: %w", err)
	}

	wg := sync.WaitGroup{}

	if c.Format != "" {
		switch c.Format {
		case dockerFormat, crioFormat, containerdFormat:
		default:
			return &Parser{}, errors.NewError(
				"operator config has an invalid `format` field.",
				"ensure that the `format` field is set to one of `docker`, `crio`, `containerd`.",
				"format", c.OnError,
			)
		}
	}

	if !removeOriginalTimeField.IsEnabled() {
		// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/33389
		set.Logger.Info("`time` log record attribute will be removed in a future release. Switch now using the feature gate.",
			zap.String("attribute", "time"),
			zap.String("feature gate", removeOriginalTimeFieldFeatureFlag),
		)
	}

	p := &Parser{
		ParserOperator:          parserOperator,
		recombineParser:         recombineParser,
		format:                  c.Format,
		addMetadataFromFilepath: c.AddMetadataFromFilePath,
		criLogEmitter:           cLogEmitter,
		criConsumers:            &wg,
	}
	return p, nil
}

// createRecombine creates an internal recombine operator which outputs to an async helper.LogEmitter
// the equivalent recombine config:
//
//	combine_field: body
//	combine_with: ""
//	is_last_entry: attributes.logtag == 'F'
//	max_log_size: 102400
//	source_identifier: attributes["log.file.path"]
//	type: recombine
func createRecombine(set component.TelemetrySettings, c Config, cLogEmitter *helper.LogEmitter) (operator.Operator, error) {
	recombineParserCfg := createRecombineConfig(c)
	recombineParser, err := recombineParserCfg.Build(set)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve internal recombine config: %w", err)
	}

	// set the LogEmmiter as the output of the recombine parser
	recombineParser.SetOutputIDs([]string{cLogEmitter.OperatorID})
	if err := recombineParser.SetOutputs([]operator.Operator{cLogEmitter}); err != nil {
		return nil, fmt.Errorf("failed to set outputs of internal recombine")
	}

	return recombineParser, nil
}

func createRecombineConfig(c Config) *recombine.Config {
	recombineParserCfg := recombine.NewConfigWithID(recombineInternalID)
	recombineParserCfg.IsLastEntry = recombineIsLastEntry
	recombineParserCfg.CombineField = entry.NewBodyField()
	recombineParserCfg.CombineWith = ""
	recombineParserCfg.SourceIdentifier = entry.NewAttributeField(recombineSourceIdentifier)
	recombineParserCfg.MaxLogSize = c.MaxLogSize
	return recombineParserCfg
}
