// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package split // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/split"

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"regexp"

	"go.uber.org/zap"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"
)

// defaultBufSize is the default size for reusable transform buffers.
// This size covers most common log line lengths. For larger inputs,
// a temporary buffer is allocated to avoid memory leaks from holding
// onto oversized buffers.
const defaultBufSize = 4096

// getBuffer returns the reusable buffer if it has sufficient capacity,
// otherwise allocates a new buffer of the needed size.
func getBuffer(reusable []byte, neededSize int) []byte {
	if neededSize <= cap(reusable) {
		return reusable[:neededSize]
	}
	return make([]byte, neededSize)
}

// Config is the configuration for a split func
type Config struct {
	LineStartPattern string `mapstructure:"line_start_pattern"`
	LineEndPattern   string `mapstructure:"line_end_pattern"`
	OmitPattern      bool   `mapstructure:"omit_pattern"`
}

// Func will return a bufio.SplitFunc based on the config
func (c Config) Func(enc encoding.Encoding, flushAtEOF bool, maxLogSize int) (bufio.SplitFunc, error) {
	return c.FuncWithLogger(enc, flushAtEOF, maxLogSize, nil)
}

// FuncWithLogger will return a bufio.SplitFunc based on the config with optional logging
func (c Config) FuncWithLogger(enc encoding.Encoding, flushAtEOF bool, maxLogSize int, logger *zap.Logger) (bufio.SplitFunc, error) {
	if enc == encoding.Nop {
		if c.LineEndPattern != "" {
			return nil, errors.New("line_end_pattern should not be set when using nop encoding")
		}
		if c.LineStartPattern != "" {
			return nil, errors.New("line_start_pattern should not be set when using nop encoding")
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
		return LineEndSplitFunc(re, c.OmitPattern, flushAtEOF, enc, logger), nil
	}

	if c.LineEndPattern == "" && c.LineStartPattern != "" {
		re, err := regexp.Compile("(?m)" + c.LineStartPattern)
		if err != nil {
			return nil, fmt.Errorf("compile line start regex: %w", err)
		}
		return LineStartSplitFunc(re, c.OmitPattern, flushAtEOF, enc, logger), nil
	}

	return nil, errors.New("only one of line_start_pattern or line_end_pattern can be set")
}

// LineStartSplitFunc creates a bufio.SplitFunc that splits an incoming stream into
// tokens that start with a match to the regex pattern provided
func LineStartSplitFunc(re *regexp.Regexp, omitPattern, flushAtEOF bool, enc encoding.Encoding, logger *zap.Logger) bufio.SplitFunc {
	// Check if encoding is UTF-8 - in this case we can match directly on bytes
	isUTF8 := enc == unicode.UTF8

	// Reusable buffer for encoding transforms to reduce allocations
	transformBuf := make([]byte, defaultBufSize)

	return func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		// Create a fresh decoder for each invocation to avoid state issues
		decoder := enc.NewDecoder()

		var firstLoc []int
		var firstMatchStart, firstMatchEnd int
		var decoded []byte

		if isUTF8 {
			// For UTF-8, match directly on bytes
			firstLoc = re.FindIndex(data)
			if firstLoc != nil {
				firstMatchStart, firstMatchEnd = firstLoc[0], firstLoc[1]
			}
		} else {
			result, flush, needMoreData := findRegexMatch(re, data, decoder, enc, atEOF, flushAtEOF, logger, transformBuf)
			if flush {
				return len(data), data, nil
			}
			if needMoreData {
				return 0, nil, nil
			}
			data = result.data
			firstLoc = result.loc
			firstMatchStart = result.matchStart
			firstMatchEnd = result.matchEnd
			decoded = result.decoded
		}

		if firstLoc == nil {
			// Flush if no more data is expected
			if len(data) != 0 && atEOF && flushAtEOF {
				return len(data), data, nil
			}
			return 0, nil, nil // read more data and try again.
		}

		if firstMatchStart != 0 {
			// the beginning of the file does not match the start pattern, so return a token up to the first match so we don't lose data
			advance = firstMatchStart
			token = data[0:firstMatchStart]

			// return if non-matching pattern is not only whitespaces
			if token != nil {
				return advance, token, err
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

		// Find second match
		var secondLoc []int
		var secondMatchStart int

		if isUTF8 {
			secondLocOfset := firstMatchEnd + 1
			secondLoc = re.FindIndex(data[secondLocOfset:])
			if secondLoc != nil {
				secondMatchStart = secondLoc[0] + secondLocOfset
			}
		} else if decoded != nil && firstLoc[1] <= len(decoded) {
			// Use the decoded data from findRegexMatch to find the second match
			remainingDecoded := decoded[firstLoc[1]:]
			secondLoc = re.FindIndex(remainingDecoded)
			if secondLoc != nil {
				// Map decoded byte position back to original encoded byte position
				matchStartBytes := remainingDecoded[:secondLoc[0]]
				encoder := enc.NewEncoder()
				neededSize := len(matchStartBytes) * 4
				buf := getBuffer(transformBuf, neededSize)
				var nDst int
				nDst, _, err = encoder.Transform(buf, matchStartBytes, true)
				if err != nil {
					return 0, nil, nil
				}
				secondMatchStart = firstMatchEnd + nDst
			}
		}

		if secondLoc == nil {
			return 0, nil, nil // read more data and try again
		}

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
func LineEndSplitFunc(re *regexp.Regexp, omitPattern, flushAtEOF bool, enc encoding.Encoding, logger *zap.Logger) bufio.SplitFunc {
	// Check if encoding is UTF-8 - in this case we can match directly on bytes
	isUTF8 := enc == unicode.UTF8

	// Reusable buffer for encoding transforms to reduce allocations
	transformBuf := make([]byte, defaultBufSize)

	return func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		// Create a fresh decoder for each invocation to avoid state issues
		decoder := enc.NewDecoder()

		var loc []int
		var matchStart, matchEnd int

		if isUTF8 {
			// For UTF-8, match directly on bytes
			loc = re.FindIndex(data)
			if loc != nil {
				matchStart, matchEnd = loc[0], loc[1]
			}
		} else {
			result, flush, needMoreData := findRegexMatch(re, data, decoder, enc, atEOF, flushAtEOF, logger, transformBuf)
			if flush {
				return len(data), data, nil
			}
			if needMoreData {
				return 0, nil, nil
			}
			data = result.data
			loc = result.loc
			matchStart = result.matchStart
			matchEnd = result.matchEnd
		}

		if loc == nil {
			// Flush if no more data is expected
			if len(data) != 0 && atEOF && flushAtEOF {
				return len(data), data, nil
			}
			return 0, nil, nil // read more data and try again
		}

		// If the match goes up to the end of the current bufer, do another
		// read until we can capture the entire match
		if matchEnd == len(data)-1 && !atEOF {
			return 0, nil, nil
		}

		if omitPattern {
			return matchEnd, data[:matchStart], nil
		}

		return matchEnd, data[:matchEnd], nil
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

// matchResult holds the result of a regex match operation on encoded data
type matchResult struct {
	loc        []int  // location in decoded string (nil if no match)
	matchStart int    // byte position of match start
	matchEnd   int    // byte position of match end
	data       []byte // potentially truncated data
	decoded    []byte // decoded data for reuse in subsequent matching
}

// findRegexMatch finds a regex match in data that may be encoded in a non-UTF8 encoding.
// It handles decoding, truncation at EOF for encodings like UTF-16, and maps the match
// positions back to byte positions in the original data.
// The transformBuf parameter is a reusable buffer for encoding transforms to reduce allocations.
// Returns:
// - result: the match result with positions mapped to byte offsets
// - flush: if true, caller should return (len(data), data, nil) to flush remaining data
// - needMoreData: if true, caller should return (0, nil, nil) to request more data
func findRegexMatch(re *regexp.Regexp, data []byte, decoder *encoding.Decoder, enc encoding.Encoding, atEOF, flushAtEOF bool, logger *zap.Logger, transformBuf []byte) (result matchResult, flush, needMoreData bool) {
	result.data = data

	decoded, decodeErr := decoder.Bytes(data)
	if decodeErr != nil {
		// If decode fails, it's likely due to incomplete data at buffer boundary
		if !atEOF {
			return result, false, true // read more data
		}
		// At EOF, if we can't decode, try to decode a truncated buffer
		// For UTF-16LE, we need even number of bytes
		truncatedLen := len(data)
		if truncatedLen%2 != 0 {
			truncatedLen--
		}
		if truncatedLen == 0 {
			// If we still can't decode, flush at EOF
			if flushAtEOF && len(data) > 0 {
				return result, true, false
			}
			return result, false, true
		}
		decoded, decodeErr = decoder.Bytes(data[:truncatedLen])
		if decodeErr != nil {
			// If we still can't decode, flush at EOF
			if flushAtEOF && len(data) > 0 {
				return result, true, false
			}
			return result, false, true
		}
		result.data = data[:truncatedLen]
	}

	result.decoded = decoded
	result.loc = re.FindIndex(decoded)
	if result.loc == nil {
		return result, false, false
	}

	// Map decoded byte positions back to original encoded byte positions
	matchStartBytes := decoded[:result.loc[0]]
	matchEndBytes := decoded[:result.loc[1]]
	encoder := enc.NewEncoder()

	// Use reusable buffer if it has sufficient capacity, otherwise allocate
	startNeeded := len(matchStartBytes) * 4
	startBuf := getBuffer(transformBuf, startNeeded)
	nDst, _, err := encoder.Transform(startBuf, matchStartBytes, true)
	if err != nil {
		// If encoding fails, fall back to UTF-8 matching
		if logger != nil {
			logger.Warn("encoding transform failed, falling back to UTF-8 matching", zap.Error(err))
		}
		result.loc = re.FindIndex(result.data)
		if result.loc != nil {
			result.matchStart, result.matchEnd = result.loc[0], result.loc[1]
		}
		return result, false, false
	}

	endNeeded := len(matchEndBytes) * 4
	endBuf := getBuffer(transformBuf, endNeeded)
	nDstEnd, _, err := encoder.Transform(endBuf, matchEndBytes, true)
	if err != nil {
		// If encoding fails, fall back to UTF-8 matching
		if logger != nil {
			logger.Warn("encoding transform failed, falling back to UTF-8 matching", zap.Error(err))
		}
		result.loc = re.FindIndex(result.data)
		if result.loc != nil {
			result.matchStart, result.matchEnd = result.loc[0], result.loc[1]
		}
		return result, false, false
	}

	result.matchStart = nDst
	result.matchEnd = nDstEnd
	return result, false, false
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
