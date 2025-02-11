// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package header // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/header"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension/experimental/storage"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/pipeline"
)

var ErrEndOfHeader = errors.New("end of header")

type Reader struct {
	set      component.TelemetrySettings
	cfg      Config
	pipeline pipeline.Pipeline
	output   *pipelineOutput
}

func NewReader(set component.TelemetrySettings, cfg Config) (*Reader, error) {
	r := &Reader{set: set, cfg: cfg}
	var err error
	r.output = newPipelineOutput(set)
	r.pipeline, err = pipeline.Config{
		Operators:     cfg.metadataOperators,
		DefaultOutput: r.output,
	}.Build(set)
	if err != nil {
		return nil, fmt.Errorf("failed to build pipeline: %w", err)
	}
	if err = r.pipeline.Start(storage.NewNopClient()); err != nil {
		return nil, fmt.Errorf("failed to start header pipeline: %w", err)
	}
	return r, nil
}

// Process checks if the given token is a line of the header, and consumes it if it is.
// An EndOfHeaderError is returned if the given line was not a header line.
func (r *Reader) Process(ctx context.Context, token []byte, fileAttributes map[string]any) error {
	if !r.cfg.regex.Match(token) {
		return ErrEndOfHeader
	}

	firstOperator := r.pipeline.Operators()[0]

	newEntry := entry.New()
	newEntry.Body = string(token)

	if err := firstOperator.Process(ctx, newEntry); err != nil {
		r.set.Logger.Error("process header entry", zap.Error(err))
		// Do not return yet. An entry was added to the logsChan which must be consumed generically.
	}

	ent, err := r.output.WaitForEntry(ctx)
	if err != nil {
		return fmt.Errorf("wait for header entry: %w", err)
	}

	// Copy resultant attributes over current set of attributes (upsert)
	for k, v := range ent.Attributes {
		// fileAttributes is an output parameter
		fileAttributes[k] = v
	}
	return nil
}

func (r *Reader) Stop() error {
	return r.pipeline.Stop()
}
