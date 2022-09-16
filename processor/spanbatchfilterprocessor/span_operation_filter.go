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

package spanbatchfilterprocessor

import (
	"crypto/sha256"
	"sort"

	"go.opentelemetry.io/collector/pdata/ptrace"
)

func filterSpansByOperationPopularity(td ptrace.Traces, mostPopular bool, tokens int) ptrace.ResourceSpansSlice {
	operations, pendingResourceSpans := sortSpansByOperation(td)
	sortedOperations := sortOperationsByPopularity(operations, pendingResourceSpans, mostPopular)
	filteredResourceSpans := filterResourceSpans(sortedOperations, pendingResourceSpans, tokens)
	return filteredResourceSpans
}

func sortSpansByOperation(td ptrace.Traces) ([]string, map[string][]ptrace.ResourceSpans) {
	operations := make([]string, 0)
	pendingResourceSpans := make(map[string][]ptrace.ResourceSpans)
	// Loop through batch of traces and sort by hashed names
	rss := td.ResourceSpans() // resource span slice
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)                     // resource span
		attrs := rs.Resource().Attributes() // save resources attributes
		sss := rs.ScopeSpans()              // scope span slice
		for j := 0; j < sss.Len(); j++ {    // loop through SERVICES
			ss := sss.At(j) // scope span
			spans := ss.Spans()
			for k := 0; k < spans.Len(); k++ {
				newResource := createEmptyResource()
				attrs.CopyTo(newResource.Resource().Attributes())
				s := spans.At(k)
				operation := s.Name()
				operationHash := hash(operation)
				s.CopyTo(newResource.ScopeSpans().At(0).Spans().At(0))
				_, exists := pendingResourceSpans[operationHash]
				if !exists {
					operations = append(operations, operationHash)
				}
				// { hash1: [resource1, resource2], hash2: [resource3] }
				pendingResourceSpans[operationHash] = append(pendingResourceSpans[operationHash], newResource)
			}
		}
	}
	return operations, pendingResourceSpans
}

func sortOperationsByPopularity(operations []string, pendingResourceSpans map[string][]ptrace.ResourceSpans, mostPopular bool) []string {
	// Sort the flows by popularity
	sort.SliceStable(operations, func(i, j int) bool {
		if mostPopular {
			return len(pendingResourceSpans[operations[i]]) > len(pendingResourceSpans[operations[j]])
		}
		return len(pendingResourceSpans[operations[i]]) < len(pendingResourceSpans[operations[j]])
	})
	return operations
}

func filterResourceSpans(sortedOperations []string, pendingResourceSpans map[string][]ptrace.ResourceSpans, totalTokens int) ptrace.ResourceSpansSlice {
	filteredResourceSpans := ptrace.NewResourceSpansSlice()
	tokens := totalTokens
	index := 0
	// add resource spans until we run out of tokens
	for tokens > 0 {
		// round robin over hashes
		for _, hash := range sortedOperations {
			if tokens < 1 {
				break
			}
			if index < len(pendingResourceSpans[hash]) {
				resource := pendingResourceSpans[hash][index]
				filteredResourceSpans.AppendEmpty()
				resource.CopyTo(filteredResourceSpans.At(totalTokens - tokens))
				tokens--
			}
		}
		index++
	}
	return filteredResourceSpans
}

func createEmptyResource() ptrace.ResourceSpans {
	newResource := ptrace.NewResourceSpans()
	newResource.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	return newResource
}

func hash(operation string) string {
	h := sha256.New()
	h.Write([]byte(operation))
	return string(h.Sum(nil))
}
