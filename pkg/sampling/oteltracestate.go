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

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	traceStateKey       = "ot"
	tValueSubkey        = "t"
	traceStateSizeLimit = 256
)

var (
	errTraceStateSyntax = fmt.Errorf("otel tracestate: %w", strconv.ErrSyntax)
)

type otelTraceState struct {
	tvalueString string
	tvalueParsed Threshold
	unknown      []string
}

func newTraceState() otelTraceState {
	return otelTraceState{
		tvalueString: "", // empty => !hasTValue(); includes "t:" prefix
	}
}

func (otts otelTraceState) serialize() string {
	var sb strings.Builder
	semi := func() {
		if sb.Len() != 0 {
			_, _ = sb.WriteString(";")
		}
	}

	if otts.hasTValue() {
		_, _ = sb.WriteString(otts.tvalueString)
	}
	for _, unk := range otts.unknown {
		ex := 0
		if sb.Len() != 0 {
			ex = 1
		}
		if sb.Len()+ex+len(unk) > traceStateSizeLimit {
			// Note: should this generate an explicit error?
			break
		}
		semi()
		_, _ = sb.WriteString(unk)
	}
	return sb.String()
}

func isValueByte(r byte) bool {
	if isLCAlphaNum(r) {
		return true
	}
	if isUCAlpha(r) {
		return true
	}
	switch r {
	case '.', '_', '-', '+':
		return true
	default:
		return false
	}
}

func isLCAlphaNum(r byte) bool {
	if isLCAlpha(r) {
		return true
	}
	return r >= '0' && r <= '9'
}

func isLCAlpha(r byte) bool {
	return r >= 'a' && r <= 'z'
}

func isUCAlpha(r byte) bool {
	return r >= 'A' && r <= 'Z'
}

func parseOTelTraceState(ts string) (otelTraceState, error) { // nolint: revive
	var tval string
	var unknown []string

	if len(ts) == 0 {
		return newTraceState(), nil
	}

	if len(ts) > traceStateSizeLimit {
		return newTraceState(), errTraceStateSyntax
	}

	for len(ts) > 0 {
		eqPos := 0
		for ; eqPos < len(ts); eqPos++ {
			if eqPos == 0 {
				if isLCAlpha(ts[eqPos]) {
					continue
				}
			} else if isLCAlphaNum(ts[eqPos]) {
				continue
			}
			break
		}
		if eqPos == 0 || eqPos == len(ts) || ts[eqPos] != ':' {
			return newTraceState(), errTraceStateSyntax
		}

		key := ts[0:eqPos]
		tail := ts[eqPos+1:]

		sepPos := 0

		for ; sepPos < len(tail); sepPos++ {
			if isValueByte(tail[sepPos]) {
				continue
			}
			break
		}

		// Here, handle recognized fields.
		if key == tValueSubkey {
			tval = ts[0 : sepPos+eqPos+1]
		} else {
			unknown = append(unknown, ts[0:sepPos+eqPos+1])
		}

		if sepPos < len(tail) && tail[sepPos] != ';' {
			return newTraceState(), errTraceStateSyntax
		}

		if sepPos == len(tail) {
			break
		}

		ts = tail[sepPos+1:]

		// test for a trailing ;
		if ts == "" {
			return newTraceState(), errTraceStateSyntax
		}
	}

	otts := newTraceState()
	otts.unknown = unknown

	if tval != "" {
		if len(tval) == 1 {
			return otts, fmt.Errorf("otel tracestate t-value: %w", strconv.ErrSyntax)
		}
		prob, _, err := TvalueToProbabilityAndAdjustedCount(tval[2:])
		if err != nil {
			return otts, fmt.Errorf("otel tracestate t-value: %w", err)
		}

		th, err := ProbabilityToThreshold(prob)
		if err != nil {
			return otts, fmt.Errorf("otel tracestate t-value: %w", err)
		}

		otts.tvalueString = tval
		otts.tvalueParsed = th
	}
	return otts, nil
}

func parseError(key string, err error) error {
	return fmt.Errorf("otel tracestate: %s-value %w", key, err)
}

func (otts otelTraceState) hasTValue() bool {
	return otts.tvalueString != ""
}
