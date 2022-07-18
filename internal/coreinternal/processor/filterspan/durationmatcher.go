// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package filterspan

import (
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterconfig"
)

type SpanDurationMatcher struct {
	Operation string
	Value     int64
}

func (ma SpanDurationMatcher) Match(span ptrace.Span) bool {
	duration := span.EndTimestamp().AsTime().Unix() - span.StartTimestamp().AsTime().Unix()

	switch ma.Operation {
	case ">":
		return duration > ma.Value
	case "<":
		return duration < ma.Value
	case "<=":
		return duration <= ma.Value
	case ">=":
		return duration >= ma.Value
	}
	return false
}

func NewDurationMatcher(config filterconfig.DurationProperties) (*SpanDurationMatcher, error) {
	entry := SpanDurationMatcher{
		Operation: config.Operator,
		Value:     config.Duration,
	}
	return &entry, nil
}
