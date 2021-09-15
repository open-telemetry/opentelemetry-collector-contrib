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

package helper

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
	return NewFlusher(c.Period)
}

// Flusher keeps information about flush state
type Flusher struct {
	// force is true when data should be flushed as soon as possible
	force bool
	// forcePeriod defines time from last flush which should pass before setting force to true.
	// Never forces if forcePeriod is set to 0
	forcePeriod time.Duration

	// lastFlush > lastForcedFlush => we can force flush if no logs are incoming for forcePeriod
	// lastFlush = lastForcedFlush => last flush was forced, so we cannot force, we can update lastFlush
	// lastFlush < lastForcedFlush => we just forced flush, set lastFlush to lastForcedFlush
	lastFlush       time.Time
	lastForcedFlush time.Time
}

// NewFlusher Creates new Flusher with lastFlush set to unix epoch
// and order to not force ongoing flush
func NewFlusher(forcePeriod Duration) *Flusher {
	return &Flusher{
		force:           false,
		lastFlush:       time.Now(),
		forcePeriod:     forcePeriod.Raw(),
		lastForcedFlush: time.Unix(0, 0),
	}
}

// Flushed update lastFlush with current timestamp
func (f *Flusher) Flushed() {
	if f.lastFlush.Sub(f.lastForcedFlush) < 0 {
		f.lastFlush = f.lastForcedFlush
	} else {
		f.lastFlush = time.Now()
	}
}

// CheckAndFlush sets internal flag to true if data is going to be force flushed
func (f *Flusher) CheckAndFlush() {
	if f.forcePeriod > 0 && time.Since(f.lastFlush) > f.forcePeriod && f.lastFlush.Sub(f.lastForcedFlush) > 0 {
		f.force = true
	}
}

// ForceFlushed update struct fields after forced flush
func (f *Flusher) ForceFlushed() {
	f.force = false
	f.lastForcedFlush = time.Now()
}

// ShouldFlush returns true if data should be forcefully flushed
func (f *Flusher) ShouldFlush() bool {
	return f.force
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
func (c MultilineConfig) Build(encoding encoding.Encoding, flushAtEOF bool, force *Flusher, maxLogSize int) (bufio.SplitFunc, error) {
	return c.getSplitFunc(encoding, flushAtEOF, force, maxLogSize)
}

// getSplitFunc returns split function for bufio.Scanner basing on configured pattern
func (c MultilineConfig) getSplitFunc(encodingVar encoding.Encoding, flushAtEOF bool, force *Flusher, maxLogSize int) (bufio.SplitFunc, error) {
	endPattern := c.LineEndPattern
	startPattern := c.LineStartPattern

	switch {
	case endPattern != "" && startPattern != "":
		return nil, fmt.Errorf("only one of line_start_pattern or line_end_pattern can be set")
	case encodingVar == encoding.Nop && (endPattern != "" || startPattern != ""):
		return nil, fmt.Errorf("line_start_pattern or line_end_pattern should not be set when using nop encoding")
	case encodingVar == encoding.Nop:
		return SplitNone(maxLogSize), nil
	case endPattern == "" && startPattern == "":
		return NewNewlineSplitFunc(encodingVar, flushAtEOF, force)
	case endPattern != "":
		re, err := regexp.Compile("(?m)" + c.LineEndPattern)
		if err != nil {
			return nil, fmt.Errorf("compile line end regex: %s", err)
		}
		return NewLineEndSplitFunc(re, flushAtEOF, force), nil
	case startPattern != "":
		re, err := regexp.Compile("(?m)" + c.LineStartPattern)
		if err != nil {
			return nil, fmt.Errorf("compile line start regex: %s", err)
		}
		return NewLineStartSplitFunc(re, flushAtEOF, force), nil
	default:
		return nil, fmt.Errorf("unreachable")
	}
}

// NewLineStartSplitFunc creates a bufio.SplitFunc that splits an incoming stream into
// tokens that start with a match to the regex pattern provided
func NewLineStartSplitFunc(re *regexp.Regexp, flushAtEOF bool, force *Flusher) bufio.SplitFunc {
	return func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if force != nil && force.ShouldFlush() {
			force.ForceFlushed()
			token = trimWhitespaces(data)
			advance = len(data)
			return
		}

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
			return
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
func NewLineEndSplitFunc(re *regexp.Regexp, flushAtEOF bool, force *Flusher) bufio.SplitFunc {
	return func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if force != nil && force.ShouldFlush() {
			force.ForceFlushed()
			token = trimWhitespaces(data)
			advance = len(data)
			return
		}
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
func NewNewlineSplitFunc(encoding encoding.Encoding, flushAtEOF bool, force *Flusher) (bufio.SplitFunc, error) {
	newline, err := encodedNewline(encoding)
	if err != nil {
		return nil, err
	}

	carriageReturn, err := encodedCarriageReturn(encoding)
	if err != nil {
		return nil, err
	}

	return func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if force != nil && force.ShouldFlush() {
			force.ForceFlushed()
			token = trimWhitespaces(data)
			advance = len(data)
			return
		}
		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}

		if i := bytes.Index(data, newline); i >= 0 {
			// We have a full newline-terminated line.
			return i + len(newline), bytes.TrimSuffix(data[:i], carriageReturn), nil
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

func encodedNewline(encoding encoding.Encoding) ([]byte, error) {
	out := make([]byte, 10)
	nDst, _, err := encoding.NewEncoder().Transform(out, []byte{'\n'}, true)
	return out[:nDst], err
}

func encodedCarriageReturn(encoding encoding.Encoding) ([]byte, error) {
	out := make([]byte, 10)
	nDst, _, err := encoding.NewEncoder().Transform(out, []byte{'\r'}, true)
	return out[:nDst], err
}

func trimWhitespaces(data []byte) []byte {
	// TrimLeft to strip EOF whitespaces in case of using $ in regex
	// For some reason newline and carriage return are being moved to beginning of next log
	// TrimRight to strip all whitespaces from the end of log
	return bytes.TrimLeft(bytes.TrimRight(data, "\r\n\t "), "\r\n")
}

// SplitterConfig consolidates MultilineConfig and FlusherConfig
type SplitterConfig struct {
	Multiline MultilineConfig `mapstructure:"multiline,omitempty"                      json:"multiline,omitempty"                     yaml:"multiline,omitempty"`
	Flusher   FlusherConfig   `mapstructure:",squash,omitempty"                        json:",inline,omitempty"                       yaml:",inline,omitempty"`
}

// NewSplitterConfig returns default SplitterConfig
func NewSplitterConfig() SplitterConfig {
	return SplitterConfig{
		Multiline: NewMultilineConfig(),
		Flusher:   NewFlusherConfig(),
	}
}

// Build builds Splitter struct
func (c *SplitterConfig) Build(encoding encoding.Encoding, flushAtEOF bool, maxLogSize int) (*Splitter, error) {
	flusher := c.Flusher.Build()
	splitFunc, err := c.Multiline.Build(encoding, flushAtEOF, flusher, maxLogSize)

	if err != nil {
		return nil, err
	}

	return &Splitter{
		Flusher:   flusher,
		SplitFunc: splitFunc,
	}, nil
}

// Splitter consolidates Flusher and dependent splitFunc
type Splitter struct {
	SplitFunc bufio.SplitFunc
	Flusher   *Flusher
}

// Flushed informs Flusher that Flushed had been performed
func (s *Splitter) Flushed() {
	s.Flusher.Flushed()
}

// CheckAndFlush instructs Flusher to check if next log should be forcefully flushed
// and set appropriate flags if yes
func (s *Splitter) CheckAndFlush() {
	s.Flusher.CheckAndFlush()
}
