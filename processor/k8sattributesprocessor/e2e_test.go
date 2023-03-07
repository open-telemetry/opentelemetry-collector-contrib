// Copyright 2020 OpenTelemetry Authors
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

//go:build e2e
// +build e2e

package k8sattributesprocessor

import (
	"bufio"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestJobE2E(t *testing.T) {
	f, err := os.Open("./testdata/trace.json")
	assert.NoError(t, err)
	defer f.Close()

	scanner := bufio.NewScanner(f)

	const maxCapacity int = 8388608
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)

	unmarshaler := ptrace.JSONUnmarshaler{}
	jobKeyFound := false
	nsKeyFound := false
	for scanner.Scan() {
		traces, err := unmarshaler.UnmarshalTraces(scanner.Bytes())
		assert.NoError(t, err)

		len := traces.ResourceSpans().Len()
		for i := 0; i < len; i++ {
			resourceSpans := traces.ResourceSpans().At(i)

			resourceSpans.Resource().Attributes().Range(
				func(k string, v pcommon.Value) bool {
					if k == "k8s.job.name" && v.Str() == "test-telemetrygen" {
						jobKeyFound = true
					}
					if k == "k8s.namespace.name" && v.Str() == "default" {
						nsKeyFound = true
					}
					return true
				},
			)
		}
	}

	assert.True(t, jobKeyFound && nsKeyFound, "job and namespace attributes are not found")

	if err := scanner.Err(); err != nil {
		assert.NoError(t, err)
	}
}
