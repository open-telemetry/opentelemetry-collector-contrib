// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package split // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/split"

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"

	"golang.org/x/text/encoding"
)

// Config is the configuration for a split func
type Config struct {
	LineStartPattern string `mapstructure:"line_start_pattern"`
	LineEndPattern   string `mapstructure:"line_end_pattern"`
	OmitPattern      bool   `mapstructure:"omit_pattern"`
}

// Func will return a bufio.SplitFunc based on the config
func (c Config) Func(enc encoding.Encoding, flushAtEOF bool, maxLogSize int) (bufio.SplitFunc, error) {
	if enc == encoding.Nop {
		if c.LineEndPattern != "" {
			return nil, fmt.Errorf("line_end_pattern should not be set when using nop encoding")
		}
		if c.LineStartPattern != "" {
			return nil, fmt.Errorf("line_start_pattern should not be set when using nop encoding")
		}
		return NoSplitFunc(maxLogSize), nil
	}

	if c.LineEndPattern == "" && c.LineStartPattern == "" {
		return NewlineSplitFunc(enc, flushAtEOF)
	}

	if c.LineEndPattern != "" && c.LineStartPattern == "" {
		re, err := regexp.Compile("(?m)" + c.LineEndPattern)
		if err != nil {
			return nil, fmt.Errorf("compile line end regex: %w", err)
		}
		return LineEndSplitFunc(re, c.OmitPattern, flushAtEOF), nil
	}

	if c.LineEndPattern == "" && c.LineStartPattern != "" {
		re, err := regexp.Compile("(?m)" + c.LineStartPattern)
		if err != nil {
			return nil, fmt.Errorf("compile line start regex: %w", err)
		}
		return LineStartSplitFunc(re, c.OmitPattern, flushAtEOF), nil
	}

	return nil, fmt.Errorf("only one of line_start_pattern or line_end_pattern can be set")
}

// LineStartSplitFunc creates a bufio.SplitFunc that splits an incoming stream into
// tokens that start with a match to the regex pattern provided
func LineStartSplitFunc(re *regexp.Regexp, omitPattern bool, flushAtEOF bool) bufio.SplitFunc {
	return func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		firstLoc := re.FindIndex(data)
		if firstLoc == nil {
			// Flush if no more data is expected
			if len(data) != 0 && atEOF && flushAtEOF {
				return len(data), data, nil
			}
			return 0, nil, nil // read more data and try again.
		}
		firstMatchStart, firstMatchEnd := firstLoc[0], firstLoc[1]

		if firstMatchStart != 0 {
			// the beginning of the file does not match the start pattern, so return a token up to the first match so we don't lose data
			advance = firstMatchStart
			token = data[0:firstMatchStart]

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
			if omitPattern {
				return len(data), data[firstMatchEnd:], nil
			}

			return len(data), data, nil
		}

		secondLocOfset := firstMatchEnd + 1
		secondLoc := re.FindIndex(data[secondLocOfset:])
		if secondLoc == nil {
			return 0, nil, nil // read more data and try again
		}
		secondMatchStart := secondLoc[0] + secondLocOfset
		if omitPattern {
			return secondMatchStart, data[firstMatchEnd:secondMatchStart], nil
		}

		// start scanning at the beginning of the second match
		// the token begins at the first match, and ends at the beginning of the second match
		return secondMatchStart, data[firstMatchStart:secondMatchStart], nil
	}
}

// LineEndSplitFunc creates a bufio.SplitFunc that splits an incoming stream into
// tokens that end with a match to the regex pattern provided
func LineEndSplitFunc(re *regexp.Regexp, omitPattern bool, flushAtEOF bool) bufio.SplitFunc {
	return func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		loc := re.FindIndex(data)
		if loc == nil {
			// Flush if no more data is expected
			if len(data) != 0 && atEOF && flushAtEOF {
				return len(data), data, nil
			}
			return 0, nil, nil // read more data and try again
		}

		// If the match goes up to the end of the current bufer, do another
		// read until we can capture the entire match
		if loc[1] == len(data)-1 && !atEOF {
			return 0, nil, nil
		}

		if omitPattern {
			return loc[1], data[:loc[0]], nil
		}

		return loc[1], data[:loc[1]], nil
	}
}

// NewlineSplitFunc splits log lines by newline, just as bufio.ScanLines, but
// never returning an token using EOF as a terminator
func NewlineSplitFunc(enc encoding.Encoding, flushAtEOF bool) (bufio.SplitFunc, error) {
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

		i := bytes.Index(data, newline)
		if i == 0 {
			return len(newline), []byte{}, nil
		}
		if i >= 0 {
			// We have a full newline-terminated line.
			token = bytes.TrimSuffix(data[:i], carriageReturn)
			return i + len(newline), token, nil
		}

		// Flush if no more data is expected
		if atEOF && flushAtEOF {
			return len(data), data, nil
		}

		// Request more data.
		return 0, nil, nil
	}, nil
}

// NoSplitFunc doesn't split any of the bytes, it reads in all of the bytes and returns it all at once. This is for when the encoding is nop
func NoSplitFunc(maxLogSize int) bufio.SplitFunc {
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
