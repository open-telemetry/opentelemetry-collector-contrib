package fileconsumer

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/pipeline"
	"go.uber.org/zap"
	"golang.org/x/text/encoding"
)

const defaultMaxHeaderSize = 1024 * 1024 // max size of 1 MiB by default
type HeaderConfig struct {
	LineStartPattern  string            `mapstructure:"multiline_pattern"`
	MetadataOperators []operator.Config `mapstructure:"metadata_operators"`
	MaxHeaderSize     *helper.ByteSize  `mapstructure:"max_size,omitempty"`
}

// validate returns an error describing why the configuration is invalid, or nil if the configuration is valid.
func (hc *HeaderConfig) validate() error {
	_, err := regexp.Compile(hc.LineStartPattern)
	if err != nil {
		return fmt.Errorf("invalid `multiline_pattern`: %w", err)
	}

	if hc.MaxHeaderSize != nil && *hc.MaxHeaderSize <= 0 {
		return errors.New("the `max_size` of the header must be greater than 0")
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

// buildHeader builds a header struct from the header config.
func (hc *HeaderConfig) buildHeader(enc encoding.Encoding, logger *zap.SugaredLogger) (*header, error) {
	reg, err := regexp.Compile(hc.LineStartPattern)
	if err != nil {
		return nil, fmt.Errorf("failed to compile multiline pattern: %w", err)
	}

	outOp := newHeaderPipelineOutput(logger)
	p, err := pipeline.Config{
		Operators:     hc.MetadataOperators,
		DefaultOutput: outOp,
	}.Build(logger)

	if err != nil {
		return nil, fmt.Errorf("failed to build pipeline: %w", err)
	}

	splitFunc, err := helper.NewNewlineSplitFunc(enc, false, func(b []byte) []byte {
		return bytes.Trim(b, "\r\n")
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create split func: %w", err)
	}

	var maxSize int
	if hc.MaxHeaderSize != nil {
		maxSize = int(*hc.MaxHeaderSize)
	} else {
		maxSize = defaultMaxHeaderSize
	}

	return &header{
		matchRegex:     reg,
		headerBytes:    &bytes.Buffer{},
		outputOperator: outOp,
		headerPipeline: p,
		logger:         logger,
		splitFunc:      splitFunc,
		maxHeaderSize:  maxSize,
	}, nil
}

type header struct {
	finalized            bool
	attributesFromHeader map[string]any

	matchRegex    *regexp.Regexp
	offset        int64
	splitFunc     bufio.SplitFunc
	maxHeaderSize int

	headerBytes    *bytes.Buffer
	outputOperator *headerPipelineOutput
	headerPipeline pipeline.Pipeline
	logger         *zap.SugaredLogger
}

// ReadHeader attempts to read the header from the given file. If the header is completed, fileAttributes will
// have its HeaderAttributes field set to the resultant header attributes.
func (h *header) ReadHeader(ctx context.Context, f io.ReadSeeker, enc helper.Encoding, fileAttributes *FileAttributes) {
	if h.finalized {
		return
	}

	// Seek to the end of the last read header line
	if _, err := f.Seek(h.offset, io.SeekStart); err != nil {
		h.logger.Errorw("Failed to seek", zap.Error(err))
		return
	}

	scanner := NewPositionalScanner(f, h.maxHeaderSize, h.offset, h.splitFunc)

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

		// If the regex doesn't match, this line is not part of the header
		if !h.matchRegex.Match(line) {
			h.finalizeHeader(ctx, fileAttributes)
			return
		}

		remainingSpace := h.maxHeaderSize - h.headerBytes.Len()
		// If writing this line to the header's buffer would cross the maximum allotted size,
		// we'll truncate to the largest size and finalize the header.
		if remainingSpace < len(line) {
			h.logger.Warnw("Max header size was reached; Truncated header is being used.", zap.Int("maxHeaderSize", h.maxHeaderSize))
			h.headerBytes.Write(line[:remainingSpace])
			// Set final offset to the end of this last line we read for the headers.
			h.offset = scanner.Pos()
			h.finalizeHeader(ctx, fileAttributes)
			return
		}

		h.headerBytes.Write(line)
		h.headerBytes.Write([]byte("\n"))
		h.offset = scanner.Pos()
	}
}

// finalizeHeader marks the header as completely read, and parses the attributes using the configured pipeline.
func (h *header) finalizeHeader(ctx context.Context, fileAttributes *FileAttributes) {
	h.finalized = true

	e, err := h.processHeaderEntry(ctx)
	if err != nil {
		h.logger.Errorw("Failed to process header", zap.Error(err))
		return
	} else {
		h.attributesFromHeader = e.Attributes
		fileAttributes.HeaderAttributes = h.attributesFromHeader
	}

	// Drop the header bytes so that the buffer can be reclaimed by the garbage collector.
	h.headerBytes = nil
}

// processHeaderEntry process the header's internal buffer as the body of an entry
func (h *header) processHeaderEntry(ctx context.Context) (*entry.Entry, error) {
	// TODO: Persister
	if err := h.headerPipeline.Start(nil); err != nil {
		return nil, fmt.Errorf("failed to start header pipeline: %w", err)
	}

	defer func() {
		if err := h.headerPipeline.Stop(); err != nil {
			h.logger.Errorw("Failed to stop header pipeline", zap.Error(err))
		}
	}()

	firstOperator := h.headerPipeline.Operators()[0]

	newEntry := entry.New()
	newEntry.Body = h.headerBytes.String()

	if err := firstOperator.Process(ctx, newEntry); err != nil {
		return nil, fmt.Errorf("failed to process header entry: %w", err)
	}

	ent, err := h.outputOperator.WaitForEntry(ctx)
	if err != nil {
		return nil, fmt.Errorf("got error while waiting for header entry: %w", err)
	}

	return ent, nil
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
