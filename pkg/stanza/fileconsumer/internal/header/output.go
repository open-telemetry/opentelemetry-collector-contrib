// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package header // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/header"

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

const pipelineOutputType = "header_log_emitter"

// pipelineOutput is a stanza operator that emits log entries to a channel
type pipelineOutput struct {
	helper.OutputOperator
	logChan chan *entry.Entry
}

// newHeaderPipelineOutput creates a new receiver output
func newPipelineOutput(logger *zap.SugaredLogger) *pipelineOutput {
	return &pipelineOutput{
		OutputOperator: helper.OutputOperator{
			BasicOperator: helper.BasicOperator{
				OperatorID:    pipelineOutputType,
				OperatorType:  pipelineOutputType,
				SugaredLogger: logger,
			},
		},
		logChan: make(chan *entry.Entry, 1),
	}
}

// Drop the entry if logChan is full, in order to avoid this operator blocking.
// This protects against a case where an operator could return an error, but continue propagating a log entry,
// leaving an unexpected entry in the output channel.
func (e *pipelineOutput) Process(_ context.Context, ent *entry.Entry) error {
	select {
	case e.logChan <- ent:
	default:
	}
	return nil
}

func (e *pipelineOutput) WaitForEntry(ctx context.Context) (*entry.Entry, error) {
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("wait for entry: %w", ctx.Err())
	case ent := <-e.logChan:
		return ent, nil
	}
}
