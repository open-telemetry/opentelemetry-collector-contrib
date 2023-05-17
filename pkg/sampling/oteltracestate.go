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

type OTelTraceState struct {
	tvalueString string
	tvalueParsed Threshold
	baseTraceState
}

type otelTraceStateParser struct{}

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

func (otts *OTelTraceState) serialize() string {
	var sb strings.Builder

	if otts.TValue() != "" {
		_, _ = sb.WriteString(otts.tvalueString)
	}

	otelSyntax.serialize(&otts.baseTraceState, &sb)

	return sb.String()
}

func (otts *OTelTraceState) HasTValue() bool {
	return otts.tvalueString != ""
}

func (otts *OTelTraceState) UnsetTValue() {
	otts.tvalueString = ""
	otts.tvalueParsed = Threshold{}
}

func (otts *OTelTraceState) TValue() string {
	return otts.tvalueString
}

func (otts *OTelTraceState) TValueThreshold() Threshold {
	return otts.tvalueParsed
}

func (otts *OTelTraceState) SetTValue(encoded string, threshold Threshold) {
	otts.tvalueString = encoded
	otts.tvalueParsed = threshold
}
