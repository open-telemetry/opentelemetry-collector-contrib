// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package stdout // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/output/stdout"

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"sync"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

// Stdout is a global handle to standard output
var Stdout io.Writer = os.Stdout

func init() {
	operator.Register("stdout", func() operator.Builder { return NewConfig("") })
}

// NewConfig creates a new stdout config with default values
func NewConfig(operatorID string) *Config {
	return &Config{
		OutputConfig: helper.NewOutputConfig(operatorID, "stdout"),
	}
}

// Config is the configuration of the Stdout operator
type Config struct {
	helper.OutputConfig `mapstructure:",squash"`
}

// Build will build a stdout operator.
func (c Config) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	outputOperator, err := c.OutputConfig.Build(logger)
	if err != nil {
		return nil, err
	}

	return &Output{
		OutputOperator: outputOperator,
		encoder:        json.NewEncoder(Stdout),
	}, nil
}

// Output is an operator that logs entries using stdout.
type Output struct {
	helper.OutputOperator
	encoder *json.Encoder
	mux     sync.Mutex
}

// Process will log entries received.
func (o *Output) Process(_ context.Context, entry *entry.Entry) error {
	o.mux.Lock()
	err := o.encoder.Encode(entry)
	if err != nil {
		o.mux.Unlock()
		o.Errorf("Failed to process entry: %s, $s", err, entry.Body)
		return err
	}
	o.mux.Unlock()
	return nil
}
