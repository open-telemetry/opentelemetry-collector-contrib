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

type W3CTraceState struct {
	otelParsed OTelTraceState
	baseTraceState
}

type w3CTraceStateParser struct{}

func NewW3CTraceState(input string) (W3CTraceState, error) {
	return w3cSyntax.parse(input)
}

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

func (wts *W3CTraceState) Serialize() string {
	var sb strings.Builder

	ots := wts.otelParsed.serialize()
	if ots != "" {
		_, _ = sb.WriteString("ot=")
		_, _ = sb.WriteString(ots)
	}

	w3cSyntax.serialize(&wts.baseTraceState, &sb)

	return sb.String()
}

func (wts *W3CTraceState) OTelValue() *OTelTraceState {
	return &wts.otelParsed
}
