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

package helper // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"

import "bufio"

// SplitterConfig consolidates MultilineConfig and FlusherConfig
type SplitterConfig struct {
	EncodingConfig EncodingConfig  `mapstructure:",squash,omitempty"`
	Flusher        FlusherConfig   `mapstructure:",squash,omitempty"`
	Multiline      MultilineConfig `mapstructure:"multiline,omitempty"`
}

// NewSplitterConfig returns default SplitterConfig
func NewSplitterConfig() SplitterConfig {
	return SplitterConfig{
		EncodingConfig: NewEncodingConfig(),
		Multiline:      NewMultilineConfig(),
		Flusher:        NewFlusherConfig(),
	}
}

// Build builds Splitter struct
func (c *SplitterConfig) Build(flushAtEOF bool, maxLogSize int) (*Splitter, error) {
	enc, err := c.EncodingConfig.Build()
	if err != nil {
		return nil, err
	}

	flusher := c.Flusher.Build()
	splitFunc, err := c.Multiline.Build(enc.Encoding, flushAtEOF, flusher, maxLogSize)
	if err != nil {
		return nil, err
	}

	return &Splitter{
		Encoding:  enc,
		Flusher:   flusher,
		SplitFunc: splitFunc,
	}, nil
}

// Splitter consolidates Flusher and dependent splitFunc
type Splitter struct {
	Encoding  Encoding
	SplitFunc bufio.SplitFunc
	Flusher   *Flusher
}

// SplitNone doesn't split any of the bytes, it reads in all of the bytes and returns it all at once. This is for when the encoding is nop
func SplitNone(maxLogSize int) bufio.SplitFunc {
	return func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if len(data) >= maxLogSize {
			return maxLogSize, data[:maxLogSize], nil
		}

		if !atEOF {
			return 0, nil, nil
		}

		if len(data) == 0 {
			return 0, nil, nil
		}
		return len(data), data, nil
	}
}
