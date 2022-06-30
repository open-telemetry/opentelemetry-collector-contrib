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
	"fmt"
	"time"

	"github.com/bmatcuk/doublestar/v3"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

const (
	defaultMaxLogSize         = 1024 * 1024
	defaultMaxConcurrentFiles = 1024
)

// NewConfig creates a new input config with default values
func NewConfig() *Config {
	return &Config{
		IncludeFileName:         true,
		IncludeFilePath:         false,
		IncludeFileNameResolved: false,
		IncludeFilePathResolved: false,
		PollInterval:            helper.Duration{Duration: 200 * time.Millisecond},
		Splitter:                helper.NewSplitterConfig(),
		StartAt:                 "end",
		FingerprintSize:         DefaultFingerprintSize,
		MaxLogSize:              defaultMaxLogSize,
		MaxConcurrentFiles:      defaultMaxConcurrentFiles,
	}
}

// Config is the configuration of a file input operator
type Config struct {
	Finder                  `mapstructure:",squash" yaml:",inline"`
	IncludeFileName         bool                  `mapstructure:"include_file_name,omitempty"              json:"include_file_name,omitempty"             yaml:"include_file_name,omitempty"`
	IncludeFilePath         bool                  `mapstructure:"include_file_path,omitempty"              json:"include_file_path,omitempty"             yaml:"include_file_path,omitempty"`
	IncludeFileNameResolved bool                  `mapstructure:"include_file_name_resolved,omitempty"     json:"include_file_name_resolved,omitempty"    yaml:"include_file_name_resolved,omitempty"`
	IncludeFilePathResolved bool                  `mapstructure:"include_file_path_resolved,omitempty"     json:"include_file_path_resolved,omitempty"    yaml:"include_file_path_resolved,omitempty"`
	PollInterval            helper.Duration       `mapstructure:"poll_interval,omitempty"                  json:"poll_interval,omitempty"                 yaml:"poll_interval,omitempty"`
	StartAt                 string                `mapstructure:"start_at,omitempty"                       json:"start_at,omitempty"                      yaml:"start_at,omitempty"`
	FingerprintSize         helper.ByteSize       `mapstructure:"fingerprint_size,omitempty"               json:"fingerprint_size,omitempty"              yaml:"fingerprint_size,omitempty"`
	MaxLogSize              helper.ByteSize       `mapstructure:"max_log_size,omitempty"                   json:"max_log_size,omitempty"                  yaml:"max_log_size,omitempty"`
	MaxConcurrentFiles      int                   `mapstructure:"max_concurrent_files,omitempty"           json:"max_concurrent_files,omitempty"          yaml:"max_concurrent_files,omitempty"`
	Splitter                helper.SplitterConfig `mapstructure:",squash,omitempty"                        json:",inline,omitempty"                       yaml:",inline,omitempty"`
}

// Build will build a file input operator from the supplied configuration
func (c Config) Build(logger *zap.SugaredLogger, emit EmitFunc) (*Input, error) {
	if emit == nil {
		return nil, fmt.Errorf("must provide emit function")
	}

	if len(c.Include) == 0 {
		return nil, fmt.Errorf("required argument `include` is empty")
	}

	// Ensure includes can be parsed as globs
	for _, include := range c.Include {
		_, err := doublestar.PathMatch(include, "matchstring")
		if err != nil {
			return nil, fmt.Errorf("parse include glob: %w", err)
		}
	}

	// Ensure excludes can be parsed as globs
	for _, exclude := range c.Exclude {
		_, err := doublestar.PathMatch(exclude, "matchstring")
		if err != nil {
			return nil, fmt.Errorf("parse exclude glob: %w", err)
		}
	}

	if c.MaxLogSize <= 0 {
		return nil, fmt.Errorf("`max_log_size` must be positive")
	}

	if c.MaxConcurrentFiles <= 1 {
		return nil, fmt.Errorf("`max_concurrent_files` must be greater than 1")
	}

	if c.FingerprintSize == 0 {
		c.FingerprintSize = DefaultFingerprintSize
	} else if c.FingerprintSize < MinFingerprintSize {
		return nil, fmt.Errorf("`fingerprint_size` must be at least %d bytes", MinFingerprintSize)
	}

	// Ensure that splitter is buildable
	_, err := c.Splitter.Build(false, int(c.MaxLogSize))
	if err != nil {
		return nil, err
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

	return &Input{
		SugaredLogger:           logger.With("component", "fileconsumer"),
		finder:                  c.Finder,
		PollInterval:            c.PollInterval.Raw(),
		captureFileName:         c.IncludeFileName,
		captureFilePath:         c.IncludeFilePath,
		captureFileNameResolved: c.IncludeFileNameResolved,
		captureFilePathResolved: c.IncludeFilePathResolved,
		startAtBeginning:        startAtBeginning,
		SplitterConfig:          c.Splitter,
		queuedMatches:           make([]string, 0),
		firstCheck:              true,
		cancel:                  func() {},
		knownFiles:              make([]*Reader, 0, 10),
		roller:                  newRoller(),
		fingerprintSize:         int(c.FingerprintSize),
		MaxLogSize:              int(c.MaxLogSize),
		MaxConcurrentFiles:      c.MaxConcurrentFiles,
		SeenPaths:               make(map[string]struct{}, 100),
		emit:                    emit,
	}, nil
}
