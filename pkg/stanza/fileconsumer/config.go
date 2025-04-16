// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package fileconsumer // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"

import (
	"bufio"
	"errors"
	"fmt"
	"runtime"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/featuregate"
	"go.uber.org/zap"
	"golang.org/x/text/encoding"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/textutils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/attrs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/emit"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/header"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/scanner"
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
		InitialBufferSize:  scanner.DefaultBufferSize,
		MaxLogSize:         reader.DefaultMaxLogSize,
		Encoding:           defaultEncoding,
		FlushPeriod:        reader.DefaultFlushPeriod,
		Resolver: attrs.Resolver{
			IncludeFileName: true,
		},
	}
}

// Config is the configuration of a file input operator
type Config struct {
	matcher.Criteria        `mapstructure:",squash"`
	attrs.Resolver          `mapstructure:",squash"`
	PollInterval            time.Duration   `mapstructure:"poll_interval,omitempty"`
	MaxConcurrentFiles      int             `mapstructure:"max_concurrent_files,omitempty"`
	MaxBatches              int             `mapstructure:"max_batches,omitempty"`
	StartAt                 string          `mapstructure:"start_at,omitempty"`
	FingerprintSize         helper.ByteSize `mapstructure:"fingerprint_size,omitempty"`
	InitialBufferSize       helper.ByteSize `mapstructure:"initial_buffer_size,omitempty"`
	MaxLogSize              helper.ByteSize `mapstructure:"max_log_size,omitempty"`
	Encoding                string          `mapstructure:"encoding,omitempty"`
	SplitConfig             split.Config    `mapstructure:"multiline,omitempty"`
	TrimConfig              trim.Config     `mapstructure:",squash,omitempty"`
	FlushPeriod             time.Duration   `mapstructure:"force_flush_period,omitempty"`
	Header                  *HeaderConfig   `mapstructure:"header,omitempty"`
	DeleteAfterRead         bool            `mapstructure:"delete_after_read,omitempty"`
	IncludeFileRecordNumber bool            `mapstructure:"include_file_record_number,omitempty"`
	Compression             string          `mapstructure:"compression,omitempty"`
	PollsToArchive          int             `mapstructure:"-"` // TODO: activate this config once archiving is set up
	AcquireFSLock           bool            `mapstructure:"acquire_fs_lock,omitempty"`
}

type HeaderConfig struct {
	Pattern           string            `mapstructure:"pattern"`
	MetadataOperators []operator.Config `mapstructure:"metadata_operators"`
}

func (c Config) Build(set component.TelemetrySettings, emit emit.Callback, opts ...Option) (*Manager, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}
	if emit == nil {
		return nil, errors.New("must provide emit function")
	}

	o := new(options)
	for _, opt := range opts {
		opt(o)
	}

	enc, err := textutils.LookupEncoding(c.Encoding)
	if err != nil {
		return nil, fmt.Errorf("failed to find encoding: %w", err)
	}

	splitFunc := o.splitFunc
	if splitFunc == nil {
		splitFunc, err = c.SplitConfig.Func(enc, false, int(c.MaxLogSize))
		if err != nil {
			return nil, err
		}
	}

	trimFunc := trim.Nop
	if enc != encoding.Nop {
		trimFunc = c.TrimConfig.Func()
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
		hCfg, err = header.NewConfig(set, c.Header.Pattern, c.Header.MetadataOperators, enc)
		if err != nil {
			return nil, fmt.Errorf("failed to build header config: %w", err)
		}
	}

	fileMatcher, err := matcher.New(c.Criteria)
	if err != nil {
		return nil, err
	}

	set.Logger = set.Logger.With(zap.String("component", "fileconsumer"))
	readerFactory := &reader.Factory{
		TelemetrySettings:       set,
		FromBeginning:           startAtBeginning,
		FingerprintSize:         int(c.FingerprintSize),
		InitialBufferSize:       int(c.InitialBufferSize),
		MaxLogSize:              int(c.MaxLogSize),
		Encoding:                enc,
		SplitFunc:               splitFunc,
		TrimFunc:                trimFunc,
		FlushTimeout:            c.FlushPeriod,
		EmitFunc:                emit,
		Attributes:              c.Resolver,
		HeaderConfig:            hCfg,
		DeleteAtEOF:             c.DeleteAfterRead,
		IncludeFileRecordNumber: c.IncludeFileRecordNumber,
		Compression:             c.Compression,
		AcquireFSLock:           c.AcquireFSLock,
	}

	telemetryBuilder, err := metadata.NewTelemetryBuilder(set)
	if err != nil {
		return nil, err
	}
	return &Manager{
		set:              set,
		readerFactory:    readerFactory,
		fileMatcher:      fileMatcher,
		pollInterval:     c.PollInterval,
		maxBatchFiles:    c.MaxConcurrentFiles / 2,
		maxBatches:       c.MaxBatches,
		telemetryBuilder: telemetryBuilder,
		noTracking:       o.noTracking,
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
		return errors.New("'max_log_size' must be positive")
	}

	if c.MaxConcurrentFiles < 1 {
		return errors.New("'max_concurrent_files' must be positive")
	}

	if c.MaxBatches < 0 {
		return errors.New("'max_batches' must not be negative")
	}

	enc, err := textutils.LookupEncoding(c.Encoding)
	if err != nil {
		return err
	}

	if c.DeleteAfterRead {
		if !allowFileDeletion.IsEnabled() {
			return fmt.Errorf("'delete_after_read' requires feature gate '%s'", allowFileDeletion.ID())
		}
		if c.StartAt == "end" {
			return errors.New("'delete_after_read' cannot be used with 'start_at: end'")
		}
	}

	if c.Header != nil {
		if !AllowHeaderMetadataParsing.IsEnabled() {
			return fmt.Errorf("'header' requires feature gate '%s'", AllowHeaderMetadataParsing.ID())
		}
		if c.StartAt == "end" {
			return errors.New("'header' cannot be specified with 'start_at: end'")
		}
		set := component.TelemetrySettings{Logger: zap.NewNop()}
		if _, errConfig := header.NewConfig(set, c.Header.Pattern, c.Header.MetadataOperators, enc); errConfig != nil {
			return fmt.Errorf("invalid config for 'header': %w", errConfig)
		}
	}

	if runtime.GOOS == "windows" && (c.IncludeFileOwnerName || c.IncludeFileOwnerGroupName) {
		return fmt.Errorf("'include_file_owner_name' or 'include_file_owner_group_name' it's not supported for windows: %w", err)
	}

	return nil
}

type options struct {
	splitFunc  bufio.SplitFunc
	noTracking bool
}

type Option func(*options)

// WithSplitFunc overrides the split func which is normally built from other settings on the config
func WithSplitFunc(f bufio.SplitFunc) Option {
	return func(o *options) {
		o.splitFunc = f
	}
}

// WithNoTracking forces the readerFactory to not keep track of files in memory. When used, the reader will
// read from the beginning of each file every time it is polled.
func WithNoTracking() Option {
	return func(o *options) {
		o.noTracking = true
	}
}
