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

package fileconsumer // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"

	"go.opentelemetry.io/collector/extension/experimental/storage"
	"go.uber.org/zap"
	"golang.org/x/text/encoding"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/pipeline"
)

type HeaderConfig struct {
	Pattern           string            `mapstructure:"pattern"`
	MetadataOperators []operator.Config `mapstructure:"metadata_operators"`

	// these are set by the "build" function
	matchRegex *regexp.Regexp
	splitFunc  bufio.SplitFunc
}

// validate returns an error describing why the configuration is invalid, or nil if the configuration is valid.
func (hc *HeaderConfig) validate() error {
	_, err := regexp.Compile(hc.Pattern)
	if err != nil {
		return fmt.Errorf("invalid `pattern`: %w", err)
	}

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

	if !p.Operators()[0].CanProcess() {
		return errors.New("first operator must be able to process entries")
	}

	return nil
}

func (hc *HeaderConfig) build(enc encoding.Encoding) error {
	var err error
	hc.matchRegex, err = regexp.Compile(hc.Pattern)
	if err != nil {
		return fmt.Errorf("failed to compile multiline pattern: %w", err)
	}

	hc.splitFunc, err = helper.NewNewlineSplitFunc(enc, false, func(b []byte) []byte {
		return bytes.Trim(b, "\r\n")
	})
	if err != nil {
		return fmt.Errorf("failed to create split func: %w", err)
	}

	return nil
}

// buildHeader builds a header struct from the header config.
func (hc *HeaderConfig) buildHeader(logger *zap.SugaredLogger) (*header, error) {
	outOp := newHeaderPipelineOutput(logger)
	p, err := pipeline.Config{
		Operators:     hc.MetadataOperators,
		DefaultOutput: outOp,
	}.Build(logger)

	if err != nil {
		return nil, fmt.Errorf("failed to build pipeline: %w", err)
	}

	if err := p.Start(storage.NewNopClient()); err != nil {
		return nil, fmt.Errorf("failed to start header pipeline: %w", err)
	}

	return &header{
		config:               hc,
		logger:               logger,
		outputOperator:       outOp,
		headerPipeline:       p,
		attributesFromHeader: map[string]any{},
	}, nil
}

type header struct {
	config *HeaderConfig
	logger *zap.SugaredLogger

	finalized            bool
	attributesFromHeader map[string]any
	offset               int64

	headerPipeline pipeline.Pipeline
	outputOperator *headerPipelineOutput
}

// ReadHeader attempts to read the header from the given file. If the header is completed, fileAttributes will
// have its HeaderAttributes field set to the resultant header attributes.
func (h *header) ReadHeader(ctx context.Context, f io.ReadSeeker, maxLineSize int, enc helper.Encoding, fileAttributes *FileAttributes) {
	if h.finalized {
		return
	}

	// Seek to the end of the last read header line
	if _, err := f.Seek(h.offset, io.SeekStart); err != nil {
		h.logger.Errorw("Failed to seek", zap.Error(err))
		return
	}

	scanner := NewPositionalScanner(f, maxLineSize, h.offset, h.config.splitFunc)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		ok := scanner.Scan()
		if !ok {
			if err := scanner.getError(); err != nil {
				h.logger.Errorw("Failed during header scan", zap.Error(err))
				h.offset = scanner.Pos()
			}
			break
		}

		b := scanner.Bytes()
		line, err := enc.Decode(b)
		if err != nil {
			h.logger.Errorw("Failed to decode header bytes", zap.Error(err))
			h.offset = scanner.Pos()
			continue
		}

		// If the regex doesn't match, we've reached the end of the header, so we'll finalize.
		if !h.config.matchRegex.Match(line) {
			h.finalizeHeader(fileAttributes)
			return
		}

		if err := h.processHeaderLine(ctx, line); err != nil {
			h.logger.Errorw("Failed to process header line", zap.Error(err))
		}
		h.offset = scanner.Pos()
	}
}

// finalizeHeader marks the header as completely read, and adds the resultant attributes to the FileAttributes.
func (h *header) finalizeHeader(fileAttributes *FileAttributes) {
	h.finalized = true
	fileAttributes.HeaderAttributes = h.attributesFromHeader
}

// processHeaderLine processes a header line, upserting entries from the line into the header attributes
func (h *header) processHeaderLine(ctx context.Context, line []byte) error {
	firstOperator := h.headerPipeline.Operators()[0]

	newEntry := entry.New()
	newEntry.Body = string(line)

	if err := firstOperator.Process(ctx, newEntry); err != nil {
		return fmt.Errorf("failed to process header entry: %w", err)
	}

	ent, err := h.outputOperator.WaitForEntry(ctx)
	if err != nil {
		return fmt.Errorf("got error while waiting for header entry: %w", err)
	}

	// Copy resultant attributes over current set of attributes (upsert)
	for k, v := range ent.Attributes {
		h.attributesFromHeader[k] = v
	}

	return nil
}

func (h *header) Shutdown() error {
	return h.headerPipeline.Stop()
}

// Finalized returns true if the header is "finalized", meaning the header has been completely
// read and processed.
func (h header) Finalized() bool {
	return h.finalized
}

// Offset returns the current offset into the file that the header has been read.
// if Finalized is true, this offset indicates where the header ends for the current file.
func (h header) Offset() int64 {
	return h.offset
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
				OperatorID:    "header_log_emitter",
				OperatorType:  "header_log_emitter",
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
	// Otherwise, we could potentially deadlock if Process is sequenced before WaitForEntry
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
