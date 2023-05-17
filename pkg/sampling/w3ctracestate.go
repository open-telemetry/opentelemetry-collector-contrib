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
	"strings"
)

// W3CTraceState represents a W3C tracestate header, which is
// organized into vendor-specific sections.  OpenTelemetry specifies
// a section that uses "ot" as the vendor key, where the t-value
// used for consistent sampling may be encoded.
//
// Note that we do not implement the limits specified in
// https://www.w3.org/TR/trace-context/#tracestate-limits because at
// this point in the traces pipeline, the tracestate is no longer
// being propagated.  Those are propagation limits, OTel does not
// specifically restrict TraceState.
//
// TODO: Should this package's tracestate support do more to implement
// those limits?  See
// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/tracestate-handling.md,
// which indicates that OTel should use a limit of 256 bytes, while
// the W3C tracestate entry as a whole recommends a limit of 512
// bytes.
type W3CTraceState struct {
	otelParsed OTelTraceState
	baseTraceState
}

// w3cSyntax describes the W3C tracestate entry.
var w3cSyntax = anyTraceStateSyntax[W3CTraceState, w3CTraceStateParser]{
	separator:  ',',
	equality:   '=',
	allowPunct: ";:._-+",
}

// w3CTraceStateParser parses tracestate strings like `k1=v1,k2=v2`
type w3CTraceStateParser struct{}

// NewW3CTraceState parses a W3C tracestate entry, especially tracking
// the OpenTelemetry entry where t-value resides for use in sampling
// decisions.
func NewW3CTraceState(input string) (W3CTraceState, error) {
	return w3cSyntax.parse(input)
}

// parseField recognizes the OpenTelemetry tracestate entry.
func (wp w3CTraceStateParser) parseField(instance *W3CTraceState, key, input string) error {
	switch {
	case key == "ot":
		value, err := stripKey(key, input)
		if err != nil {
			return err
		}

		otts, err := otelSyntax.parse(value)

		if err != nil {
			return fmt.Errorf("w3c tracestate otel value: %w", err)
		}

		instance.otelParsed = otts
		return nil
	}

	return baseTraceStateParser{}.parseField(&instance.baseTraceState, key, input)
}

// Serialize returns a W3C tracestate encoding, as would be encoded in
// a ptrace.Span.TraceState().
func (wts *W3CTraceState) Serialize() string {
	var sb strings.Builder

	ots := wts.otelParsed.serialize()
	if ots != "" {
		_, _ = sb.WriteString("ot=")
		_, _ = sb.WriteString(ots)
	}

	w3cSyntax.serializeBase(&wts.baseTraceState, &sb)

	return sb.String()
}

// OTelValue returns a reference to this value's OpenTelemetry trace
// state entry.
func (wts *W3CTraceState) OTelValue() *OTelTraceState {
	return &wts.otelParsed
}
