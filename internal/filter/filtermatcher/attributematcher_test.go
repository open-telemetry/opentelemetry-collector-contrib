// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filtermatcher // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filtermatcher"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
)

func TestMatchAttributes(t *testing.T) {
	matchCfg := filterset.Config{MatchType: filterset.Strict}
	attrsCfg := []filterconfig.Attribute{
		{Key: "strKey", Value: "strVal"},
		{Key: "intKey", Value: 1},
		{Key: "sliceKey", Value: []any{"a", "b"}},
	}
	matcher, err := NewAttributesMatcher(matchCfg, attrsCfg)
	require.NoError(t, err)

	matchingMap := pcommon.NewMap()
	matchingMap.PutStr("strKey", "strVal")
	matchingMap.PutInt("intKey", 1)
	sl := matchingMap.PutEmptySlice("sliceKey")
	sl.AppendEmpty().SetStr("a")
	sl.AppendEmpty().SetStr("b")
	matchingMap.PutStr("anotherKey", "anotherVal")

	notMatchingMap := pcommon.NewMap()
	notMatchingMap.PutStr("strKey", "strVal")
	notMatchingMap.PutInt("intKey", 1)
	notMatchingMap.PutStr("anotherKey", "anotherVal")

	assert.True(t, matcher.Match(matchingMap))
	assert.False(t, matcher.Match(notMatchingMap))
}

func BenchmarkMatchAttributes(b *testing.B) {
	matchCfg := filterset.Config{MatchType: filterset.Strict}
	attrsCfg := []filterconfig.Attribute{
		{Key: "strKey", Value: "strVal"},
		{Key: "intKey", Value: 1},
	}
	matcher, err := NewAttributesMatcher(matchCfg, attrsCfg)
	require.NoError(b, err)

	matchingMap := pcommon.NewMap()
	matchingMap.PutStr("strKey", "strVal")
	matchingMap.PutInt("intKey", 1)
	matchingMap.PutStr("anotherKey", "anotherVal")

	notMatchingMap := pcommon.NewMap()
	notMatchingMap.PutStr("strKey", "strVal")
	notMatchingMap.PutBool("boolKey", true)
	notMatchingMap.PutStr("anotherKey", "anotherVal")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matcher.Match(matchingMap)
		matcher.Match(notMatchingMap)
	}
}
