// Copyright 2022, OpenTelemetry Authors
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

package testutils // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/instanaexporter/internal/testutils"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

var (
	testAttributes = map[string]string{"instana.agent": "agent1"}
	// TestTraces traces for tests.
	TestTraces = newTracesWithAttributeMap(testAttributes)
)

func fillAttributeMap(attrs pcommon.Map, mp map[string]string) {
	attrs.Clear()
	attrs.EnsureCapacity(len(mp))
	for k, v := range mp {
		attrs.Insert(k, pcommon.NewValueString(v))
	}
}

func newTracesWithAttributeMap(mp map[string]string) ptrace.Traces {
	traces := ptrace.NewTraces()
	resourceSpans := traces.ResourceSpans()
	rs := resourceSpans.AppendEmpty()
	fillAttributeMap(rs.Resource().Attributes(), mp)
	rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	return traces
}
