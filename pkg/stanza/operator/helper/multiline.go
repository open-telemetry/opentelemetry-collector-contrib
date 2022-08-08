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

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"time"

	"golang.org/x/text/encoding"
)

// FlusherConfig is a configuration of Flusher helper
type FlusherConfig struct {
	Period Duration `mapstructure:"force_flush_period"  json:"force_flush_period" yaml:"force_flush_period"`
}

// NewFlusherConfig creates a default Flusher config
func NewFlusherConfig() FlusherConfig {
	return FlusherConfig{
		// Empty or `0s` means that we will never force flush
		Period: Duration{Duration: time.Millisecond * 500},
	}
}

// Build creates Flusher from configuration
func (c *FlusherConfig) Build() *Flusher {
	return &Flusher{
		lastDataChange:     time.Now(),
		forcePeriod:        c.Period.Raw(),
		previousDataLength: 0,
	}
}

// Flusher keeps information about flush state
type Flusher struct {
	// forcePeriod defines time from last flush which should pass before setting force to true.
	// Never forces if forcePeriod is set to 0
	forcePeriod time.Duration

	// lastDataChange tracks date of last data change (including new data and flushes)
	lastDataChange time.Time

	// previousDataLength:
	// if previousDataLength = 0 - no new data have been received after flush
	// if previousDataLength > 0 - there is data which has not been flushed yet and it doesn't changed since lastDataChange
	previousDataLength int
}

func (f *Flusher) UpdateDataChangeTime(length int) {
	// Skip if length is greater than 0 and didn't changed
	if length > 0 && length == f.previousDataLength {
		return
	}

	// update internal properties with new values if data length changed
	// because it means that data is flowing and being processed
	f.previousDataLength = length
	f.lastDataChange = time.Now()
}

// Flushed reset data length
func (f *Flusher) Flushed() {
	f.UpdateDataChangeTime(0)
}

// ShouldFlush returns true if data should be forcefully flushed
func (f *Flusher) ShouldFlush() bool {
	// Returns true if there is f.forcePeriod after f.lastDataChange and data length is greater than 0
	return f.forcePeriod > 0 && time.Since(f.lastDataChange) > f.forcePeriod && f.previousDataLength > 0
}

func (f *Flusher) SplitFunc(splitFunc bufio.SplitFunc) bufio.SplitFunc {
	return func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		advance, token, err = splitFunc(data, atEOF)

		// Return as it is in case of error
		if err != nil {
			return
		}

		// Return token
		if token != nil {
			// Inform flusher that we just flushed
			f.Flushed()
			return
		}

		// If there is no token, force flush eventually
		if f.ShouldFlush() {
			// Inform flusher that we just flushed
			f.Flushed()
			token = trimWhitespaces(data)
			advance = len(data)
			return
		}

		// Inform flusher that we didn't flushed
		f.UpdateDataChangeTime(len(data))
		return
	}
}

// Multiline consists of splitFunc and variables needed to perform force flush
type Multiline struct {
	SplitFunc bufio.SplitFunc
	Force     *Flusher
}

// NewBasicConfig creates a new Multiline config
func NewMultilineConfig() MultilineConfig {
	return MultilineConfig{
		LineStartPattern: "",
		LineEndPattern:   "",
	}
}

// MultilineConfig is the configuration of a multiline helper
type MultilineConfig struct {
	LineStartPattern string `mapstructure:"line_start_pattern"  json:"line_start_pattern" yaml:"line_start_pattern"`
	LineEndPattern   string `mapstructure:"line_end_pattern"    json:"line_end_pattern"   yaml:"line_end_pattern"`
}

// Build will build a Multiline operator.
func (c MultilineConfig) Build(enc encoding.Encoding, flushAtEOF bool, force *Flusher, maxLogSize int) (bufio.SplitFunc, error) {
	return c.getSplitFunc(enc, flushAtEOF, force, maxLogSize)
}

// getSplitFunc returns split function for bufio.Scanner basing on configured pattern
func (c MultilineConfig) getSplitFunc(enc encoding.Encoding, flushAtEOF bool, force *Flusher, maxLogSize int) (bufio.SplitFunc, error) {
	endPattern := c.LineEndPattern
	startPattern := c.LineStartPattern

	var (
		splitFunc bufio.SplitFunc
		err       error
	)

	switch {
	case endPattern != "" && startPattern != "":
		return nil, fmt.Errorf("only one of line_start_pattern or line_end_pattern can be set")
	case enc == encoding.Nop && (endPattern != "" || startPattern != ""):
		return nil, fmt.Errorf("line_start_pattern or line_end_pattern should not be set when using nop encoding")
	case enc == encoding.Nop:
		return SplitNone(maxLogSize), nil
	case endPattern == "" && startPattern == "":
		splitFunc, err = NewNewlineSplitFunc(enc, flushAtEOF)

		if err != nil {
			return nil, err
		}
	case endPattern != "":
		re, err := regexp.Compile("(?m)" + c.LineEndPattern)
		if err != nil {
			return nil, fmt.Errorf("compile line end regex: %w", err)
		}
		splitFunc = NewLineEndSplitFunc(re, flushAtEOF)
	case startPattern != "":
		re, err := regexp.Compile("(?m)" + c.LineStartPattern)
		if err != nil {
			return nil, fmt.Errorf("compile line start regex: %w", err)
		}
		splitFunc = NewLineStartSplitFunc(re, flushAtEOF)
	default:
		return nil, fmt.Errorf("unreachable")
	}

	if force != nil {
		return force.SplitFunc(splitFunc), nil
	}

	return splitFunc, nil
}

// NewLineStartSplitFunc creates a bufio.SplitFunc that splits an incoming stream into
// tokens that start with a match to the regex pattern provided
func NewLineStartSplitFunc(re *regexp.Regexp, flushAtEOF bool) bufio.SplitFunc {
	return func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		firstLoc := re.FindIndex(data)
		if firstLoc == nil {
			// Flush if no more data is expected
			if len(data) != 0 && atEOF && flushAtEOF {
				token = trimWhitespaces(data)
				advance = len(data)
				return
			}
			return 0, nil, nil // read more data and try again.
		}
		firstMatchStart := firstLoc[0]
		firstMatchEnd := firstLoc[1]

		if firstMatchStart != 0 {
			// the beginning of the file does not match the start pattern, so return a token up to the first match so we don't lose data
			advance = firstMatchStart
			token = trimWhitespaces(data[0:firstMatchStart])

			// return if non-matching pattern is not only whitespaces
			if token != nil {
				return
			}
		}

		if firstMatchEnd == len(data) {
			// the first match goes to the end of the bufer, so don't look for a second match
			return 0, nil, nil
		}

		// Flush if no more data is expected
		if atEOF && flushAtEOF {
			token = trimWhitespaces(data)
			advance = len(data)
			return
		}

		secondLocOfset := firstMatchEnd + 1
		secondLoc := re.FindIndex(data[secondLocOfset:])
		if secondLoc == nil {
			return 0, nil, nil // read more data and try again
		}
		secondMatchStart := secondLoc[0] + secondLocOfset

		advance = secondMatchStart                                      // start scanning at the beginning of the second match
		token = trimWhitespaces(data[firstMatchStart:secondMatchStart]) // the token begins at the first match, and ends at the beginning of the second match
		err = nil
		return
	}
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

// NewLineEndSplitFunc creates a bufio.SplitFunc that splits an incoming stream into
// tokens that end with a match to the regex pattern provided
func NewLineEndSplitFunc(re *regexp.Regexp, flushAtEOF bool) bufio.SplitFunc {
	return func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		loc := re.FindIndex(data)
		if loc == nil {
			// Flush if no more data is expected
			if len(data) != 0 && atEOF && flushAtEOF {
				token = trimWhitespaces(data)
				advance = len(data)
				return
			}
			return 0, nil, nil // read more data and try again
		}

		// If the match goes up to the end of the current bufer, do another
		// read until we can capture the entire match
		if loc[1] == len(data)-1 && !atEOF {
			return 0, nil, nil
		}

		advance = loc[1]
		token = trimWhitespaces(data[:loc[1]])
		err = nil
		return
	}
}

// NewNewlineSplitFunc splits log lines by newline, just as bufio.ScanLines, but
// never returning an token using EOF as a terminator
func NewNewlineSplitFunc(enc encoding.Encoding, flushAtEOF bool) (bufio.SplitFunc, error) {
	newline, err := encodedNewline(enc)
	if err != nil {
		return nil, err
	}

	carriageReturn, err := encodedCarriageReturn(enc)
	if err != nil {
		return nil, err
	}

	return func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}

		if i := bytes.Index(data, newline); i >= 0 {
			// We have a full newline-terminated line.
			token = bytes.TrimSuffix(data[:i], carriageReturn)

			return i + len(newline), trimWhitespaces(token), nil
		}

		// Flush if no more data is expected
		if atEOF && flushAtEOF {
			token = trimWhitespaces(data)
			advance = len(data)
			return
		}

		// Request more data.
		return 0, nil, nil
	}, nil
}

func encodedNewline(enc encoding.Encoding) ([]byte, error) {
	out := make([]byte, 10)
	nDst, _, err := enc.NewEncoder().Transform(out, []byte{'\n'}, true)
	return out[:nDst], err
}

func encodedCarriageReturn(enc encoding.Encoding) ([]byte, error) {
	out := make([]byte, 10)
	nDst, _, err := enc.NewEncoder().Transform(out, []byte{'\r'}, true)
	return out[:nDst], err
}

func trimWhitespaces(data []byte) []byte {
	// TrimLeft to strip EOF whitespaces in case of using $ in regex
	// For some reason newline and carriage return are being moved to beginning of next log
	// TrimRight to strip all whitespaces from the end of log
	// returns nil if log is empty
	token := bytes.TrimLeft(bytes.TrimRight(data, "\r\n\t "), "\r\n")
	if len(token) == 0 {
		return nil
	}
	return token
}

// SplitterConfig consolidates MultilineConfig and FlusherConfig
type SplitterConfig struct {
	EncodingConfig EncodingConfig  `mapstructure:",squash,omitempty"                        json:",inline,omitempty"                       yaml:",inline,omitempty"`
	Multiline      MultilineConfig `mapstructure:"multiline,omitempty"                      json:"multiline,omitempty"                     yaml:"multiline,omitempty"`
	Flusher        FlusherConfig   `mapstructure:",squash,omitempty"                        json:",inline,omitempty"                       yaml:",inline,omitempty"`
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
