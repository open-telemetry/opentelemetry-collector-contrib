// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileconsumer // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"regexp"

	"go.uber.org/zap"
	"golang.org/x/text/encoding"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/pipeline"
)

const headerPipelineOutputType = "header_log_emitter"

type HeaderConfig struct {
	Pattern           string            `mapstructure:"pattern"`
	MetadataOperators []operator.Config `mapstructure:"metadata_operators"`
}

// validate returns an error describing why the configuration is invalid, or nil if the configuration is valid.
func (hc *HeaderConfig) validate() error {
	if len(hc.MetadataOperators) == 0 {
		return errors.New("at least one operator must be specified for `metadata_operators`")
	}

	nopLogger := zap.NewNop().Sugar()
	outOp := newHeaderPipelineOutput(nopLogger)
	p, err := pipeline.Config{
		Operators:     hc.MetadataOperators,
		DefaultOutput: outOp,
	}.Build(nopLogger)

	if err != nil {
		return fmt.Errorf("failed to build pipelines: %w", err)
	}

	for _, op := range p.Operators() {
		// This is the default output we created, it's always valid
		if op.Type() == headerPipelineOutputType {
			continue
		}

		if !op.CanProcess() {
			return fmt.Errorf("operator '%s' in `metadata_operators` cannot process entries", op.ID())
		}

		if !op.CanOutput() {
			return fmt.Errorf("operator '%s' in `metadata_operators` does not propagate entries", op.ID())
		}

		// Filter processor also may fail to propagate some entries
		if op.Type() == "filter" {
			return fmt.Errorf("operator of type filter is not allowed in `metadata_operators`")
		}
	}

	return nil
}

func (hc *HeaderConfig) buildHeaderSettings(enc encoding.Encoding) (*headerSettings, error) {
	var err error
	matchRegex, err := regexp.Compile(hc.Pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to compile `pattern`: %w", err)
	}

	splitFunc, err := helper.NewNewlineSplitFunc(enc, false, func(b []byte) []byte {
		return bytes.Trim(b, "\r\n")
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create split func: %w", err)
	}

	return &headerSettings{
		matchRegex: matchRegex,
		splitFunc:  splitFunc,
		config:     hc,
	}, nil
}

// headerSettings contains compiled objects defined by a HeaderConfig
type headerSettings struct {
	matchRegex *regexp.Regexp
	splitFunc  bufio.SplitFunc
	config     *HeaderConfig
}

// headerPipelineOutput is a stanza operator that emits log entries to a channel
type headerPipelineOutput struct {
	helper.OutputOperator
	logChan chan *entry.Entry
}

// newHeaderPipelineOutput creates a new receiver output
func newHeaderPipelineOutput(logger *zap.SugaredLogger) *headerPipelineOutput {
	return &headerPipelineOutput{
		OutputOperator: helper.OutputOperator{
			BasicOperator: helper.BasicOperator{
				OperatorID:    headerPipelineOutputType,
				OperatorType:  headerPipelineOutputType,
				SugaredLogger: logger,
			},
		},
		logChan: make(chan *entry.Entry, 1),
	}
}

// Start starts the goroutine(s) required for this operator
func (e *headerPipelineOutput) Start(_ operator.Persister) error {
	return nil
}

// Stop will close the log channel and stop running goroutines
func (e *headerPipelineOutput) Stop() error {
	return nil
}

func (e *headerPipelineOutput) Process(_ context.Context, ent *entry.Entry) error {
	// Drop the entry if logChan is full, in order to avoid this operator blocking.
	// This protects against a case where an operator could return an error, but continue propagating a log entry,
	// leaving an unexpected entry in the output channel.
	select {
	case e.logChan <- ent:
	default:
	}

	return nil
}

func (e *headerPipelineOutput) WaitForEntry(ctx context.Context) (*entry.Entry, error) {
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("got context cancellation while waiting for entry: %w", ctx.Err())
	case ent := <-e.logChan:
		return ent, nil
	}
}
