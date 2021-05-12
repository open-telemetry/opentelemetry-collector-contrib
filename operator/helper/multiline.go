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

	"golang.org/x/text/encoding"

	"github.com/open-telemetry/opentelemetry-log-collection/operator"
)

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
func (c MultilineConfig) Build(context operator.BuildContext, encoding encoding.Encoding, flushAtEOF bool) (bufio.SplitFunc, error) {
	return c.getSplitFunc(encoding, flushAtEOF)
}

// getSplitFunc returns split function for bufio.Scanner basing on configured pattern
func (c MultilineConfig) getSplitFunc(encoding encoding.Encoding, flushAtEOF bool) (bufio.SplitFunc, error) {
	endPattern := c.LineEndPattern
	startPattern := c.LineStartPattern

	switch {
	case endPattern != "" && startPattern != "":
		return nil, fmt.Errorf("only one of line_start_pattern or line_end_pattern can be set")
	case endPattern == "" && startPattern == "":
		return NewNewlineSplitFunc(encoding, flushAtEOF)
	case endPattern != "":
		re, err := regexp.Compile("(?m)" + c.LineEndPattern)
		if err != nil {
			return nil, fmt.Errorf("compile line end regex: %s", err)
		}
		return NewLineEndSplitFunc(re, flushAtEOF), nil
	case startPattern != "":
		re, err := regexp.Compile("(?m)" + c.LineStartPattern)
		if err != nil {
			return nil, fmt.Errorf("compile line start regex: %s", err)
		}
		return NewLineStartSplitFunc(re, flushAtEOF), nil
	default:
		return nil, fmt.Errorf("unreachable")
	}
}

// NewLineStartSplitFunc creates a bufio.SplitFunc that splits an incoming stream into
// tokens that start with a match to the regex pattern provided
func NewLineStartSplitFunc(re *regexp.Regexp, flushAtEOF bool) bufio.SplitFunc {
	return func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		firstLoc := re.FindIndex(data)
		if firstLoc == nil {
			// Flush if no more data is expected
			if len(data) != 0 && atEOF && flushAtEOF {
				return len(data), data, nil
			}
			return 0, nil, nil // read more data and try again.
		}
		firstMatchStart := firstLoc[0]
		firstMatchEnd := firstLoc[1]

		if firstMatchStart != 0 {
			// the beginning of the file does not match the start pattern, so return a token up to the first match so we don't lose data
			advance = firstMatchStart
			token = data[0:firstMatchStart]
			return
		}

		if firstMatchEnd == len(data) {
			// the first match goes to the end of the buffer, so don't look for a second match
			return 0, nil, nil
		}

		// Flush if no more data is expected
		if atEOF && flushAtEOF {
			return len(data), data, nil
		}

		secondLocOffset := firstMatchEnd + 1
		secondLoc := re.FindIndex(data[secondLocOffset:])
		if secondLoc == nil {
			return 0, nil, nil // read more data and try again
		}
		secondMatchStart := secondLoc[0] + secondLocOffset

		advance = secondMatchStart                     // start scanning at the beginning of the second match
		token = data[firstMatchStart:secondMatchStart] // the token begins at the first match, and ends at the beginning of the second match
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
				return len(data), data, nil
			}
			return 0, nil, nil // read more data and try again
		}

		// If the match goes up to the end of the current buffer, do another
		// read until we can capture the entire match
		if loc[1] == len(data)-1 && !atEOF {
			return 0, nil, nil
		}

		advance = loc[1]
		token = data[:loc[1]]
		err = nil
		return
	}
}

// NewNewlineSplitFunc splits log lines by newline, just as bufio.ScanLines, but
// never returning an token using EOF as a terminator
func NewNewlineSplitFunc(encoding encoding.Encoding, flushAtEOF bool) (bufio.SplitFunc, error) {
	newline, err := encodedNewline(encoding)
	if err != nil {
		return nil, err
	}

	carriageReturn, err := encodedCarriageReturn(encoding)
	if err != nil {
		return nil, err
	}

	return func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}

		if i := bytes.Index(data, newline); i >= 0 {
			// We have a full newline-terminated line.
			return i + len(newline), bytes.TrimSuffix(data[:i], carriageReturn), nil
		}

		// Flush if no more data is expected
		if atEOF && flushAtEOF {
			return len(data), data, nil
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
