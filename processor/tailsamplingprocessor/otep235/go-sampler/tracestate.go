// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampler

import (
	"fmt"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// fieldSearchKey is a two-character OpenTelemetry tracestate field name
// (e.g., "rv", "th"), preceded by ';', followed by ':'.
type fieldSearchKey string

const randomnessSearchKey fieldSearchKey = ";rv:"
const thresholdSearchKey fieldSearchKey = ";th:"

// fieldPos indicates the position of the start of the key through the end of the value, ignoring the sub-key separator.
type fieldPos struct {
	start int
	end   int
}

func tracestateHasOTelField(otts string, search fieldSearchKey) (value string, savePos fieldPos, has bool) {
	var low int
	if has := strings.HasPrefix(otts, string(search[1:])); has {
		low = 3
	} else if pos := strings.Index(otts, string(search)); pos > 0 {
		low = pos + 4
	} else {
		return "", fieldPos{}, false
	}

	high := strings.IndexByte(otts[low:], ';')

	if high < 0 {
		// the field terminates at end-of-string
		high = len(otts)
	} else {
		// add the offset used above in `otts[low:]`
		high += low
	}
	start := low - 3
	return otts[low:high], fieldPos{start: start, end: high}, true
}

// tracestateHasRandomness determines whether there is a "rv" sub-key
func tracestateHasRandomness(otts string) (int64, bool) {
	val, _, has := tracestateHasOTelField(otts, randomnessSearchKey)
	if !has {
		return 0, false
	}
	if len(val) != 14 {
		otel.Handle(fmt.Errorf("could not parse tracestate randomness: %q: %w", otts, strconv.ErrSyntax))
		return 0, false
	}
	rv, err := strconv.ParseUint(val, 16, 64)
	if err != nil {
		otel.Handle(fmt.Errorf("could not parse tracestate randomness: %q: %w", val, err))
		return 0, false
	}
	return int64(rv), true
}

// tracestateHasThreshold determines whether there is a "th" sub-key
func tracestateHasThreshold(otts string) (int64, fieldPos, bool) {
	val, savePos, has := tracestateHasOTelField(otts, thresholdSearchKey)
	if !has {
		return 0, fieldPos{}, false
	}
	if len(val) == 0 || len(val) > 14 {
		otel.Handle(fmt.Errorf("could not parse tracestate threshold: %q: %w", otts, strconv.ErrSyntax))
		return -1, fieldPos{}, false
	}
	th, err := strconv.ParseUint(val, 16, 64)
	if err != nil {
		otel.Handle(fmt.Errorf("could not parse tracestate threshold: %q: %w", val, err))
		return -1, fieldPos{}, false
	}
	// Add trailing zeros
	th <<= (14 - len(val)) * 4
	return int64(th), savePos, true
}

var simpleAlwaysSampleTracestate = func() trace.TraceState {
	rts, _ := trace.ParseTraceState("ot=th:0")
	return rts
}()

func updateOT(original trace.TraceState, out string) (trace.TraceState, error) {
	if out == "" {
		return original.Delete("ot"), nil
	}
	return original.Insert("ot", out)
}

// combineTracestate combines an existing OTel tracestate fragment,
// which is the value of a top-level "ot" tracestate vendor tag.
func combineTracestate(original trace.TraceState, updateThreshold int64, thresholdReliable bool, parsedThreshold int64, thPos fieldPos, hasThreshold bool) (trace.TraceState, error) {
	// Try to optimize several fast paths. Remember this is a prototype :-)
	switch {
	case !thresholdReliable && !hasThreshold && parsedThreshold == 0:
		// No threshold in, no threshold out.
		return original, nil
	case original.Len() == 0 && updateThreshold == 0:
		// Empty tracestate, 100% sampling case.
		return simpleAlwaysSampleTracestate, nil
	case thresholdReliable && hasThreshold && parsedThreshold == updateThreshold:
		// Unchanged (e.g., due to ParentThreshold)
		return original, nil
	}

	// By design the OT tracestate value is unmodified.
	// Note: Maybe trim whitespace from the value below?
	unmodified := original.Get("ot")

	var out strings.Builder

	copyExceptThreshold := func() int {
		sections := 0

		// Case where we erase a threshold.
		//
		// hadThreshold is true, otherwise the branch
		// above. Write all except the former threshold.
		start := thPos.start
		end := thPos.end

		// If the th sub-key is not first, subtract 1 to consume a
		// separator.
		if start != 0 {
			start--
			_, _ = out.WriteString(unmodified[:start])
			sections++
		}
		// If the th sub-key is not last, [end:] includes a separator.
		if end != len(unmodified) {
			_, _ = out.WriteString(unmodified[end:])
			sections++
		}
		return sections
	}

	if !thresholdReliable {
		copyExceptThreshold()
		return updateOT(original, out.String())
	}
	// Sections is zero or non-zero, absolute value does not matter.
	var sections int
	if thPos.start != thPos.end {
		sections = copyExceptThreshold()
	} else {
		_, _ = out.WriteString(unmodified)
		sections = len(unmodified)
	}
	nf := ";th:"
	if sections == 0 {
		nf = nf[1:]
	}
	_, _ = out.WriteString(nf)

	if updateThreshold == 0 {
		// Special case is required, otherwise the TrimRight() below
		// would leave an empty string.
		out.WriteString("0")
	} else {
		// Format as an unsigned integer and remove trailing zeros.
		out.WriteString(strings.TrimRight(strconv.FormatUint(uint64(updateThreshold), 16), "0"))
	}
	return updateOT(original, out.String())
}
