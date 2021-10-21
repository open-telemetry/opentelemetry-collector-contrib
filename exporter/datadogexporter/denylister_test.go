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

package datadogexporter

import (
	"testing"

	"github.com/DataDog/datadog-agent/pkg/trace/exportable/pb"
	"github.com/stretchr/testify/assert"
)

// TestSpan returns a fix span with hardcoded info, useful for reproducible tests
// https://github.com/DataDog/datadog-agent/blob/d3a4cd66314d70162e2c76d2481f4b93455cf717/pkg/trace/test/testutil/span.go#L372
func testSpan() *pb.Span {
	return &pb.Span{
		Duration: 10000000,
		Error:    0,
		Resource: "GET /some/raclette",
		Service:  "django",
		Name:     "django.controller",
		SpanID:   42,
		Start:    1472732573337575936,
		TraceID:  424242,
		Meta: map[string]string{
			"user": "eric",
			"pool": "fondue",
		},
		Metrics: map[string]float64{
			"cheese_weight": 100000.0,
		},
		ParentID: 1111,
		Type:     "http",
	}
}

func TestDenylister(t *testing.T) {
	tests := []struct {
		filter      []string
		resource    string
		expectation bool
	}{
		{[]string{"/foo/bar"}, "/foo/bar", false},
		{[]string{"/foo/b.r"}, "/foo/bar", false},
		{[]string{"[0-9]+"}, "/abcde", true},
		{[]string{"[0-9]+"}, "/abcde123", false},
		{[]string{"\\(foobar\\)"}, "(foobar)", false},
		{[]string{"\\(foobar\\)"}, "(bar)", true},
		{[]string{"(GET|POST) /healthcheck"}, "GET /foobar", true},
		{[]string{"(GET|POST) /healthcheck"}, "GET /healthcheck", false},
		{[]string{"(GET|POST) /healthcheck"}, "POST /healthcheck", false},
		{[]string{"SELECT COUNT\\(\\*\\) FROM BAR"}, "SELECT COUNT(*) FROM BAR", false},
		{[]string{"ABC+", "W+"}, "ABCCCC", false},
		{[]string{"ABC+", "W+"}, "WWW", false},
	}

	for _, test := range tests {
		span := testSpan()
		span.Resource = test.resource
		filter := newDenylister(test.filter)

		assert.Equal(t, test.expectation, filter.allows(span))
	}
}

func TestCompileRules(t *testing.T) {
	filter := newDenylister([]string{"\n{6}"})
	for i := 0; i < 100; i++ {
		span := testSpan()
		assert.True(t, filter.allows(span))
	}
}
