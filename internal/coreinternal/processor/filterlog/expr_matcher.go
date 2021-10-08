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

package filterlog

import (
	"go.opentelemetry.io/collector/model/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterexpr"
)

type exprMatcher struct {
	matchers []*filterexpr.LogMatcher
}

func newExprMatcher(expressions []string) (*exprMatcher, error) {
	m := &exprMatcher{}
	for _, expression := range expressions {
		matcher, err := filterexpr.NewLogMatcher(expression)
		if err != nil {
			return nil, err
		}
		m.matchers = append(m.matchers, matcher)
	}
	return m, nil
}

func (m *exprMatcher) MatchLog(lr pdata.LogRecord, resource pdata.Resource, library pdata.InstrumentationLibrary) bool {
	return m.MatchLogRecord(lr)
}

func (m *exprMatcher) MatchLogRecord(lr pdata.LogRecord) bool {
	for _, matcher := range m.matchers {
		matched, err := matcher.MatchLog(lr)
		if err != nil {
			return false
		}
		if matched {
			return true
		}
	}
	return false
}
