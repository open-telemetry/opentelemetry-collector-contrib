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

	"golang.org/x/text/encoding"
)

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
	LineStartPattern string `mapstructure:"line_start_pattern"`
	LineEndPattern   string `mapstructure:"line_end_pattern"`
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
	token := bytes.TrimLeft(bytes.TrimRight(data, "\r\n\t "), "\r\n")
	if token == nil {
		return []byte{}
	}
	return token
}
