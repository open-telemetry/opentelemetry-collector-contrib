// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package goldendataset // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/goldendataset"

import (
	"fmt"
	"io"
	"math/rand"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// GenerateTraces generates a slice of OTLP ResourceSpans objects based on the PICT-generated pairwise
// parameters defined in the parameters file specified by the tracePairsFile parameter. The pairs to generate
// spans for for defined in the file specified by the spanPairsFile parameter.
// The slice of ResourceSpans are returned. If an err is returned, the slice elements will be nil.
func GenerateTraces(tracePairsFile string, spanPairsFile string) ([]ptrace.Traces, error) {
	random := io.Reader(rand.New(rand.NewSource(42)))
	pairsData, err := loadPictOutputFile(tracePairsFile)
	if err != nil {
		return nil, err
	}
	pairsTotal := len(pairsData) - 1
	traces := make([]ptrace.Traces, pairsTotal)
	for index, values := range pairsData {
		if index == 0 {
			continue
		}
		tracingInputs := &PICTTracingInputs{
			Resource:               PICTInputResource(values[TracesColumnResource]),
			InstrumentationLibrary: PICTInputInstrumentationLibrary(values[TracesColumnInstrumentationLibrary]),
			Spans:                  PICTInputSpans(values[TracesColumnSpans]),
		}
		traces[index-1] = ptrace.NewTraces()
		spanErr := appendResourceSpan(tracingInputs, spanPairsFile, random, traces[index-1].ResourceSpans())
		if spanErr != nil {
			return nil, err
		}
	}
	return traces, err
}

// generateResourceSpan generates a single PData ResourceSpans populated based on the provided inputs. They are:
//   tracingInputs - the pairwise combination of field value variations for this ResourceSpans
//   spanPairsFile - the file with the PICT-generated parameter combinations to generate spans for
//   random - the random number generator to use in generating ID values
//
// The generated resource spans. If err is not nil, some or all of the resource spans fields will be nil.
func appendResourceSpan(tracingInputs *PICTTracingInputs, spanPairsFile string,
	random io.Reader, resourceSpansSlice ptrace.ResourceSpansSlice) error {
	resourceSpan := resourceSpansSlice.AppendEmpty()
	err := appendScopeSpans(tracingInputs, spanPairsFile, random, resourceSpan.ScopeSpans())
	if err != nil {
		return err
	}
	GenerateResource(tracingInputs.Resource).CopyTo(resourceSpan.Resource())
	return nil
}

func appendScopeSpans(tracingInputs *PICTTracingInputs, spanPairsFile string,
	random io.Reader, scopeSpansSlice ptrace.ScopeSpansSlice) error {
	var count int
	switch tracingInputs.InstrumentationLibrary {
	case LibraryNone:
		count = 1
	case LibraryOne:
		count = 1
	case LibraryTwo:
		count = 2
	}
	for i := 0; i < count; i++ {
		err := fillScopeSpans(tracingInputs, i, spanPairsFile, random, scopeSpansSlice.AppendEmpty())
		if err != nil {
			return err
		}
	}
	return nil
}

func fillScopeSpans(tracingInputs *PICTTracingInputs, index int, spanPairsFile string, random io.Reader, scopeSpans ptrace.ScopeSpans) error {
	spanCaseCount, err := countTotalSpanCases(spanPairsFile)
	if err != nil {
		return err
	}
	fillInstrumentationLibrary(tracingInputs, index, scopeSpans.Scope())
	switch tracingInputs.Spans {
	case LibrarySpansNone:
		return nil
	case LibrarySpansOne:
		return appendSpans(1, spanPairsFile, random, scopeSpans.Spans())
	case LibrarySpansSeveral:
		return appendSpans(spanCaseCount/4, spanPairsFile, random, scopeSpans.Spans())
	case LibrarySpansAll:
		return appendSpans(spanCaseCount, spanPairsFile, random, scopeSpans.Spans())
	default:
		return appendSpans(16, spanPairsFile, random, scopeSpans.Spans())
	}
}

func countTotalSpanCases(spanPairsFile string) (int, error) {
	pairsData, err := loadPictOutputFile(spanPairsFile)
	if err != nil {
		return 0, err
	}
	count := len(pairsData) - 1
	return count, err
}

func fillInstrumentationLibrary(tracingInputs *PICTTracingInputs, index int, scope pcommon.InstrumentationScope) {
	if tracingInputs.InstrumentationLibrary == LibraryNone {
		return
	}
	nameStr := fmt.Sprintf("%s-%s-%s-%d", tracingInputs.Resource, tracingInputs.InstrumentationLibrary, tracingInputs.Spans, index)
	verStr := "semver:1.1.7"
	if index > 0 {
		verStr = ""
	}
	scope.SetName(nameStr)
	scope.SetVersion(verStr)
}
