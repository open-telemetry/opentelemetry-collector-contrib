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

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type booleanAttributeFilter struct {
	key    string
	value  bool
	logger *zap.Logger
}

var _ PolicyEvaluator = (*booleanAttributeFilter)(nil)

// NewBooleanAttributeFilter creates a policy evaluator that samples all traces with
// the given attribute that match the supplied boolean value.
func NewBooleanAttributeFilter(logger *zap.Logger, key string, value bool) PolicyEvaluator {
	return &booleanAttributeFilter{
		key:    key,
		value:  value,
		logger: logger,
	}
}

// Evaluate looks at the trace data and returns a corresponding SamplingDecision.
func (baf *booleanAttributeFilter) Evaluate(_ pcommon.TraceID, trace *TraceData) (Decision, error) {
	trace.Lock()
	batches := trace.ReceivedBatches
	trace.Unlock()

	return hasResourceOrSpanWithCondition(
		batches,
		func(resource pcommon.Resource) bool {
			if v, ok := resource.Attributes().Get(baf.key); ok {
				value := v.Bool()
				return value == baf.value
			}
			return false
		},
		func(span ptrace.Span) bool {
			if v, ok := span.Attributes().Get(baf.key); ok {
				value := v.Bool()
				return value == baf.value
			}
			return false
		}), nil
}
