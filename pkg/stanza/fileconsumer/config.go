// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileconsumer // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"

import (
	"bufio"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/featuregate"
	"go.uber.org/zap"
	"golang.org/x/text/encoding"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/decode"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/emit"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/header"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/matcher"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/split"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/trim"
)

const (
	defaultMaxConcurrentFiles = 1024
	defaultEncoding           = "utf-8"
	defaultPollInterval       = 200 * time.Millisecond
)

var allowFileDeletion = featuregate.GlobalRegistry().MustRegister(
	"filelog.allowFileDeletion",
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription("When enabled, allows usage of the `delete_after_read` setting."),
	featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/16314"),
)

var AllowHeaderMetadataParsing = featuregate.GlobalRegistry().MustRegister(
	"filelog.allowHeaderMetadataParsing",
	featuregate.StageBeta,
	featuregate.WithRegisterDescription("When enabled, allows usage of the `header` setting."),
	featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/18198"),
)

// NewConfig creates a new input config with default values
func NewConfig() *Config {
	return &Config{
		PollInterval:       defaultPollInterval,
		MaxConcurrentFiles: defaultMaxConcurrentFiles,
		StartAt:            "end",
		FingerprintSize:    fingerprint.DefaultSize,
		MaxLogSize:         reader.DefaultMaxLogSize,
		Encoding:           defaultEncoding,
		FlushPeriod:        reader.DefaultFlushPeriod,
		IncludeFileName:    true,
	}
}

// Config is the configuration of a file input operator
type Config struct {
	matcher.Criteria        `mapstructure:",squash"`
	PollInterval            time.Duration   `mapstructure:"poll_interval,omitempty"`
	MaxConcurrentFiles      int             `mapstructure:"max_concurrent_files,omitempty"`
	MaxBatches              int             `mapstructure:"max_batches,omitempty"`
	StartAt                 string          `mapstructure:"start_at,omitempty"`
	FingerprintSize         helper.ByteSize `mapstructure:"fingerprint_size,omitempty"`
	MaxLogSize              helper.ByteSize `mapstructure:"max_log_size,omitempty"`
	Encoding                string          `mapstructure:"encoding,omitempty"`
	SplitConfig             split.Config    `mapstructure:"multiline,omitempty"`
	TrimConfig              trim.Config     `mapstructure:",squash,omitempty"`
	FlushPeriod             time.Duration   `mapstructure:"force_flush_period,omitempty"`
	IncludeFileName         bool            `mapstructure:"include_file_name,omitempty"`
	IncludeFilePath         bool            `mapstructure:"include_file_path,omitempty"`
	IncludeFileNameResolved bool            `mapstructure:"include_file_name_resolved,omitempty"`
	IncludeFilePathResolved bool            `mapstructure:"include_file_path_resolved,omitempty"`
	Header                  *HeaderConfig   `mapstructure:"header,omitempty"`
	DeleteAfterRead         bool            `mapstructure:"delete_after_read,omitempty"`
}

type HeaderConfig struct {
	Pattern           string            `mapstructure:"pattern"`
	MetadataOperators []operator.Config `mapstructure:"metadata_operators"`
}

// Build will build a file input operator from the supplied configuration
func (c Config) Build(logger *zap.SugaredLogger, emit emit.Callback) (*Manager, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}

	enc, err := decode.LookupEncoding(c.Encoding)
	if err != nil {
		return nil, err
	}

	splitFunc, err := c.SplitConfig.Func(enc, false, int(c.MaxLogSize))
	if err != nil {
		return nil, err
	}

	trimFunc := trim.Nop
	if enc != encoding.Nop {
		trimFunc = c.TrimConfig.Func()
	}

	return c.buildManager(logger, emit, splitFunc, trimFunc)
}

// BuildWithSplitFunc will build a file input operator with customized splitFunc function
func (c Config) BuildWithSplitFunc(logger *zap.SugaredLogger, emit emit.Callback, splitFunc bufio.SplitFunc) (*Manager, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}
	return c.buildManager(logger, emit, splitFunc, c.TrimConfig.Func())
}

func (c Config) buildManager(logger *zap.SugaredLogger, emit emit.Callback, splitFunc bufio.SplitFunc, trimFunc trim.Func) (*Manager, error) {
	if emit == nil {
		return nil, fmt.Errorf("must provide emit function")
	}
	var startAtBeginning bool
	switch c.StartAt {
	case "beginning":
		startAtBeginning = true
	case "end":
		startAtBeginning = false
	default:
		return nil, fmt.Errorf("invalid start_at location '%s'", c.StartAt)
	}

	enc, err := decode.LookupEncoding(c.Encoding)
	if err != nil {
		return nil, fmt.Errorf("failed to find encoding: %w", err)
	}

	var hCfg *header.Config
	if c.Header != nil {
		hCfg, err = header.NewConfig(c.Header.Pattern, c.Header.MetadataOperators, enc)
		if err != nil {
			return nil, fmt.Errorf("failed to build header config: %w", err)
		}
	}

	fileMatcher, err := matcher.New(c.Criteria)
	if err != nil {
		return nil, err
	}

	readerFactory := reader.Factory{
		SugaredLogger:           logger.With("component", "fileconsumer"),
		FromBeginning:           startAtBeginning,
		FingerprintSize:         int(c.FingerprintSize),
		MaxLogSize:              int(c.MaxLogSize),
		Encoding:                enc,
		SplitFunc:               splitFunc,
		TrimFunc:                trimFunc,
		FlushTimeout:            c.FlushPeriod,
		EmitFunc:                emit,
		IncludeFileName:         c.IncludeFileName,
		IncludeFilePath:         c.IncludeFilePath,
		IncludeFileNameResolved: c.IncludeFileNameResolved,
		IncludeFilePathResolved: c.IncludeFilePathResolved,
		HeaderConfig:            hCfg,
		DeleteAtEOF:             c.DeleteAfterRead,
	}

	return &Manager{
		SugaredLogger:     logger.With("component", "fileconsumer"),
		readerFactory:     readerFactory,
		fileMatcher:       fileMatcher,
		pollInterval:      c.PollInterval,
		maxBatchFiles:     c.MaxConcurrentFiles / 2,
		maxBatches:        c.MaxBatches,
		previousPollFiles: make([]*reader.Reader, 0, c.MaxConcurrentFiles/2),
		knownFiles:        []*reader.Metadata{},
	}, nil
}

func (c Config) validate() error {
	if _, err := matcher.New(c.Criteria); err != nil {
		return err
	}

	if c.FingerprintSize < fingerprint.MinSize {
		return fmt.Errorf("'fingerprint_size' must be at least %d bytes", fingerprint.MinSize)
	}

	if c.MaxLogSize <= 0 {
		return fmt.Errorf("'max_log_size' must be positive")
	}

	if c.MaxConcurrentFiles <= 1 {
		return fmt.Errorf("'max_concurrent_files' must be positive")
	}

	if c.MaxBatches < 0 {
		return errors.New("'max_batches' must not be negative")
	}

	enc, err := decode.LookupEncoding(c.Encoding)
	if err != nil {
		return err
	}

	if c.DeleteAfterRead {
		if !allowFileDeletion.IsEnabled() {
			return fmt.Errorf("'delete_after_read' requires feature gate '%s'", allowFileDeletion.ID())
		}
		if c.StartAt == "end" {
			return fmt.Errorf("'delete_after_read' cannot be used with 'start_at: end'")
		}
	}

	if c.Header != nil {
		if !AllowHeaderMetadataParsing.IsEnabled() {
			return fmt.Errorf("'header' requires feature gate '%s'", AllowHeaderMetadataParsing.ID())
		}
		if c.StartAt == "end" {
			return fmt.Errorf("'header' cannot be specified with 'start_at: end'")
		}
		if _, err := header.NewConfig(c.Header.Pattern, c.Header.MetadataOperators, enc); err != nil {
			return fmt.Errorf("invalid config for 'header': %w", err)
		}
	}

	return nil
}
