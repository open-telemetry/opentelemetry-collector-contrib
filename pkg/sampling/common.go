package sampling

import (
	"errors"
	"strings"

	"go.uber.org/multierr"
)

type KV struct {
	Key   string
	Value string
}

var (
	ErrTraceStateSize  = errors.New("invalid tracestate size")
	ErrTraceStateCount = errors.New("invalid tracestate item count")
)

// keyValueScanner defines distinct scanner behaviors for lists of
// key-values.
type keyValueScanner struct {
	// maxItems is 32 or -1
	maxItems int
	// trim is set if OWS (optional whitespace) should be removed
	trim bool
	// separator is , or ;
	separator byte
	// equality is = or :
	equality byte
}

type commonTraceState struct {
	kvs []KV
}

func (cts commonTraceState) HasExtraValues() bool {
	return len(cts.kvs) != 0
}

func (cts commonTraceState) ExtraValues() []KV {
	return cts.kvs
}

// trimOws removes optional whitespace on both ends of a string.
func trimOws(input string) string {
	// Hard-codes the value of owsCharset
	for len(input) > 0 && input[0] == ' ' || input[0] == '\t' {
		input = input[1:]
	}
	for len(input) > 0 && input[len(input)-1] == ' ' || input[len(input)-1] == '\t' {
		input = input[:len(input)-1]
	}
	return input
}

func (s keyValueScanner) scanKeyValues(input string, f func(key, value string) error) error {
	var rval error
	items := 0
	for input != "" {
		items++
		if s.maxItems > 0 && items >= s.maxItems {
			// W3C specifies max 32 entries, tested here
			// instead of via the regexp.
			return ErrTraceStateCount
		}

		sep := strings.IndexByte(input, s.separator)

		var member string
		if sep < 0 {
			member = input
			input = ""
		} else {
			member = input[:sep]
			input = input[sep+1:]
		}

		if s.trim {
			// Trim only required for W3C; OTel does not
			// specify whitespace for its value encoding.
			member = trimOws(member)
		}

		if member == "" {
			// W3C allows empty list members.
			continue
		}

		eq := strings.IndexByte(member, s.equality)
		if eq < 0 {
			// A regexp should have rejected this input.
			continue
		}
		if err := f(member[:eq], member[eq+1:]); err != nil {
			rval = multierr.Append(rval, err)
		}
	}
	return rval
}
