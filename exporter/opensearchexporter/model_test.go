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

package opensearchexporter

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"testing"
)

func Test_encodeModel_encodeSpan(t *testing.T) {
	type args struct {
		resource pcommon.Resource
		span     ptrace.Span
	}
	tests := []struct {
		name    string
		args    args
		want    []byte
		wantErr assert.ErrorAssertionFunc
	}{
		// TODO: Add test cases.
		{
			name:    "simple",
			args:    args{resource: newResource(), span: sampleSpan()},
			want:    nil,
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &encodeModel{
				dedup: true,
				dedot: false,
			}
			trace := ptrace.NewTraces()
			rs := trace.ResourceSpans().AppendEmpty()
			rs.Resource().Attributes().PutStr("test", "rest")
			ss := rs.ScopeSpans().AppendEmpty()
			es := ss.Spans().AppendEmpty()
			es.SetSpanID(pcommon.SpanID{1, 0, 0, 0})
			es.Events().AppendEmpty().SetName("an_event")
			jm := ptrace.JSONMarshaler{}
			marshalled, err := jm.MarshalTraces(trace)
			assert.NotEmpty(t, marshalled)

			got, err := m.encodeSpan(tt.args.resource, tt.args.span)
			if !tt.wantErr(t, err, fmt.Sprintf("encodeSpan(%v, %v)", tt.args.resource, tt.args.span)) {
				return
			}
			assert.Equalf(t, tt.want, got, "encodeSpan(%v, %v)", tt.args.resource, tt.args.span)
		})
	}
}

func newResource() pcommon.Resource {
	r := pcommon.NewResource()
	r.Attributes().PutStr("test", "rest")
	return r
}

func sampleSpan() ptrace.Span {
	sp := ptrace.NewSpan()

	sp.SetSpanID(pcommon.SpanID{1, 0, 0, 0})
	return sp
}
