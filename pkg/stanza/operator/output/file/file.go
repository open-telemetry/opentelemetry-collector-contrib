// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package file // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/output/file"

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

var operatorType = component.MustNewType("file_output")

func init() {
	operator.RegisterFactory(NewFactory())
}

// Deprecated [v0.97.0] Use Factory.NewDefaultConfig instead.
func NewConfig(operatorID string) *Config {
	return NewFactory().NewDefaultConfig(operatorID).(*Config)
}

// Config is the configuration of a file output operatorn.
type Config struct {
	helper.OutputConfig `mapstructure:",squash"`

	Path   string `mapstructure:"path"`
	Format string `mapstructure:"format"`
}

// Deprecated [v0.97.0] Use NewFactory.CreateOperator instead.
func (c Config) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	return NewFactory().CreateOperator(&c, component.TelemetrySettings{Logger: logger.Desugar()})
}

// NewFactory creates a new factory.
func NewFactory() operator.Factory {
	return operator.NewFactory(operatorType, newDefaultConfig, createOperator)
}

func newDefaultConfig(operatorID string) component.Config {
	return &Config{
		OutputConfig: helper.NewOutputConfig(operatorID, "file_output"),
	}
}

func createOperator(cfg component.Config, set component.TelemetrySettings) (operator.Operator, error) {
	c := cfg.(*Config)
	outputOperator, err := helper.NewOutputOperator(c.OutputConfig, set)
	if err != nil {
		return nil, err
	}

	var tmpl *template.Template
	if c.Format != "" {
		tmpl, err = template.New("file").Parse(c.Format)
		if err != nil {
			return nil, err
		}
	}

	if c.Path == "" {
		return nil, fmt.Errorf("must provide a path to output to")
	}

	return &Output{
		OutputOperator: outputOperator,
		path:           c.Path,
		tmpl:           tmpl,
	}, nil
}

// Output is an operator that writes logs to a file.
type Output struct {
	helper.OutputOperator

	path    string
	tmpl    *template.Template
	encoder *json.Encoder
	file    *os.File
	mux     sync.Mutex
}

// Start will open the output file.
func (fo *Output) Start(_ operator.Persister) error {
	var err error
	fo.file, err = os.OpenFile(fo.path, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0600)
	if err != nil {
		return err
	}

	fo.encoder = json.NewEncoder(fo.file)
	fo.encoder.SetEscapeHTML(false)

	return nil
}

// Stop will close the output file.
func (fo *Output) Stop() error {
	if fo.file != nil {
		if err := fo.file.Close(); err != nil {
			fo.Errorf(err.Error())
		}
	}
	return nil
}

// Process will write an entry to the output file.
func (fo *Output) Process(_ context.Context, entry *entry.Entry) error {
	fo.mux.Lock()
	defer fo.mux.Unlock()

	if fo.tmpl != nil {
		err := fo.tmpl.Execute(fo.file, entry)
		if err != nil {
			return err
		}
	} else {
		err := fo.encoder.Encode(entry)
		if err != nil {
			return err
		}
	}

	return nil
}
