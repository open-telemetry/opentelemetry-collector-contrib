// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"fmt"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSortUniqInPlace(t *testing.T) {
	elements := []string{"tag3:tagggg", "tag2:tagval", "tag1:tagval", "tag2:tagval"}
	elements = UniqInPlace(elements)

	assert.ElementsMatch(t, elements, []string{"tag1:tagval", "tag2:tagval", "tag3:tagggg"})
}

func benchmarkDeduplicateTags(b *testing.B, numberOfTags int) {
	tags := make([]string, 0, numberOfTags+1)
	for i := 0; i < numberOfTags; i++ {
		tags = append(tags, fmt.Sprintf("aveeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeerylong:tag%d", i))
	}
	// this is the worst case for the insertion sort we are using
	sort.Sort(sort.Reverse(sort.StringSlice(tags)))

	tempTags := make([]string, len(tags))
	copy(tempTags, tags)
	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		copy(tempTags, tags)
		UniqInPlace(tempTags)
	}
}

func BenchmarkDeduplicateTags(b *testing.B) {
	for i := 1; i <= 128; i *= 2 {
		b.Run(fmt.Sprintf("deduplicate-%d-tags-in-place", i), func(b *testing.B) {
			benchmarkDeduplicateTags(b, i)
		})
	}
}
