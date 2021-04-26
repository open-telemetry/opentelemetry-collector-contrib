// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package datadogexporter

import (
	"testing"

	"github.com/DataDog/datadog-agent/pkg/trace/exportable/pb"
	"github.com/stretchr/testify/assert"
)

// https://github.com/DataDog/datadog-agent/blob/d3a4cd66314d70162e2c76d2481f4b93455cf717/pkg/trace/test/testutil/span.go#L372
// TestSpan returns a fix span with hardcoded info, useful for reproducible tests
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

func TestBlocklister(t *testing.T) {
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
		{[]string{"[123"}, "[123", true},
		{[]string{"\\[123"}, "[123", false},
		{[]string{"ABC+", "W+"}, "ABCCCC", false},
		{[]string{"ABC+", "W+"}, "WWW", false},
	}

	for _, test := range tests {
		span := testSpan()
		span.Resource = test.resource
		filter := NewBlocklister(test.filter)

		assert.Equal(t, test.expectation, filter.Allows(span))
	}
}

func TestCompileRules(t *testing.T) {
	filter := NewBlocklister([]string{"[123", "]123", "{6}"})
	for i := 0; i < 100; i++ {
		span := testSpan()
		assert.True(t, filter.Allows(span))
	}
}
