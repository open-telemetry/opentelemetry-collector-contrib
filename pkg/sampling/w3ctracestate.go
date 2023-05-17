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

type w3CTraceState struct {
	otelString string
	otelParsed otelTraceState
	baseTraceState
}

type w3CTraceStateParser struct{}

func (wp w3CTraceStateParser) parseField(concrete *w3CTraceState, key, input string) error {
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

		concrete.otelString = input
		concrete.otelParsed = otts
		return nil
	}

	return baseTraceStateParser{}.parseField(&concrete.baseTraceState, key, input)
}

func (wts w3CTraceState) serialize() string {
	var sb strings.Builder

	if wts.hasOTelValue() {
		_, _ = sb.WriteString(wts.otelString)
	}

	w3cSyntax.serialize(&wts.baseTraceState, &sb)

	return sb.String()
}

func (wts w3CTraceState) hasOTelValue() bool {
	return wts.otelString != ""
}
