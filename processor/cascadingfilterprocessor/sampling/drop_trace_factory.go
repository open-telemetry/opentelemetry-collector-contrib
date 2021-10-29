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

package sampling

import (
	"regexp"

	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/cascadingfilterprocessor/config"
)

type dropTraceEvaluator struct {
	numericAttr *numericAttributeFilter
	stringAttr  *stringAttributeFilter
	attrs       []attributeFilter
	operationRe *regexp.Regexp

	logger *zap.Logger
}

var _ DropTraceEvaluator = (*dropTraceEvaluator)(nil)

// NewDropTraceEvaluator creates a drop trace evaluator that checks if trace should be dropped
func NewDropTraceEvaluator(logger *zap.Logger, cfg config.TraceRejectCfg) (DropTraceEvaluator, error) {
	numericAttrFilter := createNumericAttributeFilter(cfg.NumericAttributeCfg)
	stringAttrFilter, err := createStringAttributeFilter(cfg.StringAttributeCfg)
	if err != nil {
		return nil, err
	}
	attrsFilter, err := createAttributesFilter(cfg.AttributeCfg)
	if err != nil {
		return nil, err
	}

	var operationRe *regexp.Regexp

	if cfg.NamePattern != nil {
		operationRe, err = regexp.Compile(*cfg.NamePattern)
		if err != nil {
			return nil, err
		}
	}

	return &dropTraceEvaluator{
		stringAttr:  stringAttrFilter,
		numericAttr: numericAttrFilter,
		attrs:       attrsFilter,
		operationRe: operationRe,
		logger:      logger,
	}, nil
}

// ShouldDrop checks if trace should be dropped
func (dte *dropTraceEvaluator) ShouldDrop(_ pdata.TraceID, trace *TraceData) bool {
	trace.Lock()
	batches := trace.ReceivedBatches
	trace.Unlock()

	matchingOperationFound := false
	matchingStringAttrFound := false
	matchingNumericAttrFound := false
	matchingAttrsFound := false

	for _, batch := range batches {
		rs := batch.ResourceSpans()

		for i := 0; i < rs.Len(); i++ {
			res := rs.At(i).Resource()

			if !matchingStringAttrFound && dte.stringAttr != nil {
				matchingStringAttrFound = checkIfStringAttrFound(res.Attributes(), dte.stringAttr)
			}
			if !matchingNumericAttrFound && dte.numericAttr != nil {
				matchingNumericAttrFound = checkIfNumericAttrFound(res.Attributes(), dte.numericAttr)
			}

			ils := rs.At(i).InstrumentationLibrarySpans()
			for j := 0; j < ils.Len(); j++ {
				spans := ils.At(j).Spans()
				for k := 0; k < spans.Len(); k++ {
					span := spans.At(k)

					if !matchingAttrsFound && len(dte.attrs) > 0 {
						matchingAttrsFound = checkIfAttrsMatched(res.Attributes(), span.Attributes(), dte.attrs)
					}
					if !matchingStringAttrFound && dte.stringAttr != nil {
						matchingStringAttrFound = checkIfStringAttrFound(span.Attributes(), dte.stringAttr)
					}
					if !matchingNumericAttrFound && dte.numericAttr != nil {
						matchingNumericAttrFound = checkIfNumericAttrFound(span.Attributes(), dte.numericAttr)
					}

					if dte.operationRe != nil && !matchingOperationFound {
						if dte.operationRe.MatchString(span.Name()) {
							matchingOperationFound = true
						}
					}
				}
			}
		}
	}

	conditionMet := struct {
		operationName, stringAttr, numericAttr, attrs bool
	}{
		operationName: true,
		stringAttr:    true,
		numericAttr:   true,
		attrs:         true,
	}

	if dte.operationRe != nil {
		conditionMet.operationName = matchingOperationFound
	}
	if dte.numericAttr != nil {
		conditionMet.numericAttr = matchingNumericAttrFound
	}
	if dte.stringAttr != nil {
		conditionMet.stringAttr = matchingStringAttrFound
	}
	if len(dte.attrs) > 0 {
		conditionMet.attrs = matchingAttrsFound
	}

	return conditionMet.operationName && conditionMet.numericAttr && conditionMet.stringAttr && conditionMet.attrs
}
