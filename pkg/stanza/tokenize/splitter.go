// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tokenize // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/tokenize"

import (
	"bufio"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/decode"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/trim"
)

// SplitterConfig consolidates MultilineConfig and FlusherConfig
type SplitterConfig struct {
	Encoding         string          `mapstructure:"encoding,omitempty"`
	Flusher          FlusherConfig   `mapstructure:",squash,omitempty"`
	Multiline        MultilineConfig `mapstructure:"multiline,omitempty"`
	PreserveLeading  bool            `mapstructure:"preserve_leading_whitespaces,omitempty"`
	PreserveTrailing bool            `mapstructure:"preserve_trailing_whitespaces,omitempty"`
}

// NewSplitterConfig returns default SplitterConfig
func NewSplitterConfig() SplitterConfig {
	return SplitterConfig{
		Encoding:  "utf-8",
		Multiline: NewMultilineConfig(),
		Flusher:   FlusherConfig{Period: DefaultFlushPeriod},
	}
}

// Build builds bufio.SplitFunc based on the config
func (c *SplitterConfig) Build(flushAtEOF bool, maxLogSize int) (bufio.SplitFunc, error) {
	enc, err := decode.LookupEncoding(c.Encoding)
	if err != nil {
		return nil, err
	}
	trimFunc := trim.Whitespace(c.PreserveLeading, c.PreserveTrailing)
	splitFunc, err := c.Multiline.Build(enc, flushAtEOF, maxLogSize, trimFunc)
	if err != nil {
		return nil, err
	}

	return c.Flusher.Wrap(splitFunc, trimFunc), nil
}
