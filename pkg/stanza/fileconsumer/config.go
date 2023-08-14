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

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/emit"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/header"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/matcher"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

const (
	defaultMaxLogSize         = 1024 * 1024
	defaultMaxConcurrentFiles = 1024
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
		IncludeFileName:         true,
		IncludeFilePath:         false,
		IncludeFileNameResolved: false,
		IncludeFilePathResolved: false,
		PollInterval:            200 * time.Millisecond,
		Splitter:                helper.NewSplitterConfig(),
		StartAt:                 "end",
		FingerprintSize:         fingerprint.DefaultSize,
		MaxLogSize:              defaultMaxLogSize,
		MaxConcurrentFiles:      defaultMaxConcurrentFiles,
		MaxBatches:              0,
	}
}

// Config is the configuration of a file input operator
type Config struct {
	matcher.Criteria        `mapstructure:",squash"`
	IncludeFileName         bool                  `mapstructure:"include_file_name,omitempty"`
	IncludeFilePath         bool                  `mapstructure:"include_file_path,omitempty"`
	IncludeFileNameResolved bool                  `mapstructure:"include_file_name_resolved,omitempty"`
	IncludeFilePathResolved bool                  `mapstructure:"include_file_path_resolved,omitempty"`
	PollInterval            time.Duration         `mapstructure:"poll_interval,omitempty"`
	StartAt                 string                `mapstructure:"start_at,omitempty"`
	FingerprintSize         helper.ByteSize       `mapstructure:"fingerprint_size,omitempty"`
	MaxLogSize              helper.ByteSize       `mapstructure:"max_log_size,omitempty"`
	MaxConcurrentFiles      int                   `mapstructure:"max_concurrent_files,omitempty"`
	MaxBatches              int                   `mapstructure:"max_batches,omitempty"`
	DeleteAfterRead         bool                  `mapstructure:"delete_after_read,omitempty"`
	Splitter                helper.SplitterConfig `mapstructure:",squash,omitempty"`
	Header                  *HeaderConfig         `mapstructure:"header,omitempty"`
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

	// Ensure that splitter is buildable
	factory := newMultilineSplitterFactory(c.Splitter)
	if _, err := factory.Build(int(c.MaxLogSize)); err != nil {
		return nil, err
	}

	return c.buildManager(logger, emit, factory)
}

// BuildWithSplitFunc will build a file input operator with customized splitFunc function
func (c Config) BuildWithSplitFunc(logger *zap.SugaredLogger, emit emit.Callback, splitFunc bufio.SplitFunc) (*Manager, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}

	if splitFunc == nil {
		return nil, fmt.Errorf("must provide split function")
	}

	// Ensure that splitter is buildable
	factory := newCustomizeSplitterFactory(c.Splitter.Flusher, splitFunc)
	if _, err := factory.Build(int(c.MaxLogSize)); err != nil {
		return nil, err
	}

	return c.buildManager(logger, emit, factory)
}

func (c Config) buildManager(logger *zap.SugaredLogger, emit emit.Callback, factory splitterFactory) (*Manager, error) {
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

	var hCfg *header.Config
	if c.Header != nil {
		enc, err := helper.LookupEncoding(c.Splitter.EncodingConfig.Encoding)
		if err != nil {
			return nil, fmt.Errorf("failed to create encoding: %w", err)
		}

		hCfg, err = header.NewConfig(c.Header.Pattern, c.Header.MetadataOperators, enc)
		if err != nil {
			return nil, fmt.Errorf("failed to build header config: %w", err)
		}
	}

	fileMatcher, err := matcher.New(c.Criteria)
	if err != nil {
		return nil, err
	}

	return &Manager{
		SugaredLogger: logger.With("component", "fileconsumer"),
		cancel:        func() {},
		readerFactory: readerFactory{
			SugaredLogger: logger.With("component", "fileconsumer"),
			readerConfig: &readerConfig{
				fingerprintSize:         int(c.FingerprintSize),
				maxLogSize:              int(c.MaxLogSize),
				emit:                    emit,
				includeFileName:         c.IncludeFileName,
				includeFilePath:         c.IncludeFilePath,
				includeFileNameResolved: c.IncludeFileNameResolved,
				includeFilePathResolved: c.IncludeFilePathResolved,
			},
			fromBeginning:   startAtBeginning,
			splitterFactory: factory,
			encodingConfig:  c.Splitter.EncodingConfig,
			headerConfig:    hCfg,
		},
		fileMatcher:     fileMatcher,
		roller:          newRoller(),
		pollInterval:    c.PollInterval,
		maxBatchFiles:   c.MaxConcurrentFiles / 2,
		maxBatches:      c.MaxBatches,
		deleteAfterRead: c.DeleteAfterRead,
		knownFiles:      make([]*reader, 0, 10),
		seenPaths:       make(map[string]struct{}, 100),
	}, nil
}

func (c Config) validate() error {
	if c.DeleteAfterRead && !allowFileDeletion.IsEnabled() {
		return fmt.Errorf("`delete_after_read` requires feature gate `%s`", allowFileDeletion.ID())
	}

	if c.Header != nil && !AllowHeaderMetadataParsing.IsEnabled() {
		return fmt.Errorf("`header` requires feature gate `%s`", AllowHeaderMetadataParsing.ID())
	}

	if _, err := matcher.New(c.Criteria); err != nil {
		return err
	}

	if c.MaxLogSize <= 0 {
		return fmt.Errorf("`max_log_size` must be positive")
	}

	if c.MaxConcurrentFiles <= 1 {
		return fmt.Errorf("`max_concurrent_files` must be greater than 1")
	}

	if c.FingerprintSize < fingerprint.MinSize {
		return fmt.Errorf("`fingerprint_size` must be at least %d bytes", fingerprint.MinSize)
	}

	if c.DeleteAfterRead && c.StartAt == "end" {
		return fmt.Errorf("`delete_after_read` cannot be used with `start_at: end`")
	}

	if c.Header != nil && c.StartAt == "end" {
		return fmt.Errorf("`header` cannot be specified with `start_at: end`")
	}

	if c.MaxBatches < 0 {
		return errors.New("`max_batches` must not be negative")
	}

	enc, err := helper.LookupEncoding(c.Splitter.EncodingConfig.Encoding)
	if err != nil {
		return err
	}

	if c.Header != nil {
		if _, err := header.NewConfig(c.Header.Pattern, c.Header.MetadataOperators, enc); err != nil {
			return fmt.Errorf("invalid config for `header`: %w", err)
		}
	}

	return nil
}
