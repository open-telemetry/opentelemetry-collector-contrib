// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package utils // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/utils"

import (
	"sort"
)

// UniqInPlace sorts and removes duplicates from elements in place.
// The returned slice is a subslice of elements.
// Copied from datadog-agent utils package
// https://github.com/DataDog/datadog-agent/blob/3e244d1d8ac9a6d4ed86e42036056f7153a27fe4/pkg/util/sort/sort_uniq.go
func UniqInPlace(elements []string) []string {
	if len(elements) < 2 {
		return elements
	}

	// Sort the slice
	sort.Strings(elements)

	// Remove duplicates
	return uniqSorted(elements)
}

// uniqSorted removes duplicate elements from the given slice.
// The given slice needs to be sorted.
func uniqSorted(elements []string) []string {
	if len(elements) == 0 {
		return elements
	}

	j := 0
	for i := 1; i < len(elements); i++ {
		if elements[j] == elements[i] {
			continue
		}
		j++
		elements[j] = elements[i]
	}
	return elements[:j+1]
}
