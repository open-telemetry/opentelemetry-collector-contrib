// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileconsumer // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"

import (
	"bufio"
	"errors"
	"fmt"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"go.opentelemetry.io/collector/featuregate"
	"go.uber.org/zap"

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
	featuregate.StageAlpha,
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
		FingerprintSize:         DefaultFingerprintSize,
		MaxLogSize:              defaultMaxLogSize,
		MaxConcurrentFiles:      defaultMaxConcurrentFiles,
		MaxBatches:              0,
	}
}

// Config is the configuration of a file input operator
type Config struct {
	Finder                  `mapstructure:",squash"`
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

// Build will build a file input operator from the supplied configuration
func (c Config) Build(logger *zap.SugaredLogger, emit EmitFunc) (*Manager, error) {
	if c.DeleteAfterRead && !allowFileDeletion.IsEnabled() {
		return nil, fmt.Errorf("`delete_after_read` requires feature gate `%s`", allowFileDeletion.ID())
	}

	if c.Header != nil && !AllowHeaderMetadataParsing.IsEnabled() {
		return nil, fmt.Errorf("`header` requires feature gate `%s`", AllowHeaderMetadataParsing.ID())
	}

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
func (c Config) BuildWithSplitFunc(
	logger *zap.SugaredLogger, emit EmitFunc, splitFunc bufio.SplitFunc) (*Manager, error) {
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

func (c Config) buildManager(logger *zap.SugaredLogger, emit EmitFunc, factory splitterFactory) (*Manager, error) {
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

	var hs *headerSettings
	if c.Header != nil {
		enc, err := c.Splitter.EncodingConfig.Build()
		if err != nil {
			return nil, fmt.Errorf("failed to create encoding: %w", err)
		}

		hs, err = c.Header.buildHeaderSettings(enc.Encoding)
		if err != nil {
			return nil, fmt.Errorf("failed to build header config: %w", err)
		}
	}

	return &Manager{
		SugaredLogger: logger.With("component", "fileconsumer"),
		cancel:        func() {},
		readerFactory: readerFactory{
			SugaredLogger: logger.With("component", "fileconsumer"),
			readerConfig: &readerConfig{
				fingerprintSize: int(c.FingerprintSize),
				maxLogSize:      int(c.MaxLogSize),
				emit:            emit,
			},
			fromBeginning:   startAtBeginning,
			splitterFactory: factory,
			encodingConfig:  c.Splitter.EncodingConfig,
			headerSettings:  hs,
		},
		finder:          c.Finder,
		roller:          newRoller(),
		pollInterval:    c.PollInterval,
		maxBatchFiles:   c.MaxConcurrentFiles / 2,
		maxBatches:      c.MaxBatches,
		deleteAfterRead: c.DeleteAfterRead,
		knownFiles:      make([]*Reader, 0, 10),
		seenPaths:       make(map[string]struct{}, 100),
	}, nil
}

func (c Config) validate() error {
	if len(c.Include) == 0 {
		return fmt.Errorf("required argument `include` is empty")
	}

	// Ensure includes can be parsed as globs
	for _, include := range c.Include {
		_, err := doublestar.PathMatch(include, "matchstring")
		if err != nil {
			return fmt.Errorf("parse include glob: %w", err)
		}
	}

	// Ensure excludes can be parsed as globs
	for _, exclude := range c.Exclude {
		_, err := doublestar.PathMatch(exclude, "matchstring")
		if err != nil {
			return fmt.Errorf("parse exclude glob: %w", err)
		}
	}

	if c.MaxLogSize <= 0 {
		return fmt.Errorf("`max_log_size` must be positive")
	}

	if c.MaxConcurrentFiles <= 1 {
		return fmt.Errorf("`max_concurrent_files` must be greater than 1")
	}

	if c.FingerprintSize < MinFingerprintSize {
		return fmt.Errorf("`fingerprint_size` must be at least %d bytes", MinFingerprintSize)
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

	_, err := c.Splitter.EncodingConfig.Build()
	if err != nil {
		return err
	}

	if c.Header != nil {
		if err := c.Header.validate(); err != nil {
			return fmt.Errorf("invalid config for `header`: %w", err)
		}
	}

	return nil
}
