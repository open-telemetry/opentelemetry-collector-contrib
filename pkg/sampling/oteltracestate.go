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

// OTelTraceState parses the sampling t-value.  It accumulates other,
// unrecognized fields in the baseTraceState object.
type OTelTraceState struct {
	tvalueString string
	tvalueParsed Threshold
	baseTraceState
}

// otelSyntax describes the OTel trace state entry.
var otelSyntax = anyTraceStateSyntax[OTelTraceState, otelTraceStateParser]{
	separator:  ';',
	equality:   ':',
	allowPunct: "._-+",
}

// otelTraceStateParser parses tracestate strings like `k1:v1;k2:v2`
type otelTraceStateParser struct{}

// parseField recognizes and parses t-value entries.
func (wp otelTraceStateParser) parseField(instance *OTelTraceState, key, input string) error {
	switch {
	case key == "t":
		value, err := stripKey(key, input)
		if err != nil {
			return err
		}

		prob, _, err := TvalueToProbabilityAndAdjustedCount(value)
		if err != nil {
			return fmt.Errorf("otel tracestate t-value: %w", err)
		}

		th, err := ProbabilityToThreshold(prob)
		if err != nil {
			return fmt.Errorf("otel tracestate t-value: %w", err)
		}

		instance.tvalueString = input
		instance.tvalueParsed = th

		return nil
	}

	return baseTraceStateParser{}.parseField(&instance.baseTraceState, key, input)
}

// serialize generates the OTel tracestate encoding.  Called by W3CTraceState.Serialize.
func (otts *OTelTraceState) serialize() string {
	var sb strings.Builder

	if otts.TValue() != "" {
		_, _ = sb.WriteString(otts.tvalueString)
	}

	otelSyntax.serializeBase(&otts.baseTraceState, &sb)

	return sb.String()
}

// HasTValue indicates whether a non-empty t-value was received.
func (otts *OTelTraceState) HasTValue() bool {
	return otts.tvalueString != ""
}

// UnsetTValue clears the t-value, generally meant for use when the
// t-value is inconsistent.
func (otts *OTelTraceState) UnsetTValue() {
	otts.tvalueString = ""
	otts.tvalueParsed = Threshold{}
}

// TValue returns a whole encoding, including the leading "t:".
func (otts *OTelTraceState) TValue() string {
	return otts.tvalueString
}

// TValueThreshold returns the threshold used given the parsed t-value.
func (otts *OTelTraceState) TValueThreshold() Threshold {
	return otts.tvalueParsed
}

// SetTValue modifies the t-value.  The user should supply the correct
// new threshold, it will not be re-calculated.
func (otts *OTelTraceState) SetTValue(encoded string, threshold Threshold) {
	otts.tvalueString = encoded
	otts.tvalueParsed = threshold
}
