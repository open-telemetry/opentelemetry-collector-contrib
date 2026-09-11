// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package httpcheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/httpcheckreceiver"

import (
	"fmt"
	"strings"
	"testing"
)

// benchBody builds a JSON-ish response body of roughly the given size.
func benchBody(approxSize int) []byte {
	var sb strings.Builder
	sb.WriteString(`{"status":"ok","items":[`)
	for i := 0; sb.Len() < approxSize; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		fmt.Fprintf(&sb, `{"id":%d,"name":"item-%d","state":"active"}`, i, i)
	}
	sb.WriteString(`]}`)
	return []byte(sb.String())
}

func BenchmarkValidateResponse(b *testing.B) {
	body := benchBody(4096)

	cases := []struct {
		name        string
		validations []validationConfig
	}{
		{
			name:        "regex",
			validations: []validationConfig{{Regex: `"state":"active"`}},
		},
		{
			name: "mixed",
			validations: []validationConfig{
				{Contains: "ok"},
				{JSONPath: "status", Equals: "ok"},
				{Regex: `"state":"(active|pending)"`},
			},
		},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			target := &targetConfig{Validations: tc.validations}
			if err := target.compileValidations(); err != nil {
				b.Fatal(err)
			}

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				validateResponse(body, target.Validations)
			}
		})
	}
}
