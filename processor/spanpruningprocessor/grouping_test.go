// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package spanpruningprocessor

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestWriteAttributeValueKey_Double(t *testing.T) {
	builder := &strings.Builder{}

	value := pcommon.NewValueDouble(12.5)
	writeAttributeValueKey(builder, value)

	assert.Equal(t, "Double:12.5", builder.String())
}

func TestWriteAttributeValueKey_Bytes(t *testing.T) {
	builder := &strings.Builder{}

	bytesValue := []byte{0x01, 0x02, 0x03, 0xff}

	value := pcommon.NewValueBytes()
	value.Bytes().FromRaw(bytesValue)

	writeAttributeValueKey(builder, value)

	expected := "Bytes:" + base64.StdEncoding.EncodeToString(bytesValue)
	assert.Equal(t, expected, builder.String())
}

func TestWriteAttributeValueKey_Map(t *testing.T) {
	builder := &strings.Builder{}

	value := pcommon.NewValueMap()
	value.Map().PutStr("service", "api")
	value.Map().PutInt("port", 8080)

	writeAttributeValueKey(builder, value)

	assert.Equal(
		t,
		"Map:{port=Int:8080,service=Str:api}",
		builder.String(),
	)
}

func TestWriteAttributeValueKey_Slice(t *testing.T) {
	builder := &strings.Builder{}

	value := pcommon.NewValueSlice()
	slice := value.Slice()

	slice.AppendEmpty().SetStr("first")
	slice.AppendEmpty().SetInt(42)

	writeAttributeValueKey(builder, value)

	assert.Equal(
		t,
		"Slice:[Str:first,Int:42]",
		builder.String(),
	)
}

func TestWriteAttributeMapKey_SortsKeys(t *testing.T) {
	builder := &strings.Builder{}

	value := pcommon.NewValueMap()
	value.Map().PutInt("z", 3)
	value.Map().PutInt("a", 1)
	value.Map().PutInt("m", 2)

	writeAttributeMapKey(builder, value.Map())

	assert.Equal(
		t,
		"{a=Int:1,m=Int:2,z=Int:3}",
		builder.String(),
	)
}

func TestWriteAttributeMapKey_NestedValues(t *testing.T) {
	builder := &strings.Builder{}

	value := pcommon.NewValueMap()
	nestedMap := value.Map().PutEmptyMap("nested")

	nestedMap.PutStr("b", "two")
	nestedMap.PutStr("a", "one")

	value.Map().PutStr("name", "test")

	writeAttributeMapKey(builder, value.Map())

	assert.Equal(
		t,
		"{name=Str:test,nested=Map:{a=Str:one,b=Str:two}}",
		builder.String(),
	)
}

func TestWriteAttributeSliceKey_PreservesOrder(t *testing.T) {
	first := pcommon.NewValueSlice()
	first.Slice().AppendEmpty().SetStr("first")
	first.Slice().AppendEmpty().SetStr("second")

	second := pcommon.NewValueSlice()
	second.Slice().AppendEmpty().SetStr("second")
	second.Slice().AppendEmpty().SetStr("first")

	firstBuilder := &strings.Builder{}
	secondBuilder := &strings.Builder{}

	writeAttributeSliceKey(firstBuilder, first.Slice())
	writeAttributeSliceKey(secondBuilder, second.Slice())

	assert.Equal(t, "[Str:first,Str:second]", firstBuilder.String())
	assert.Equal(t, "[Str:second,Str:first]", secondBuilder.String())
	assert.NotEqual(t, firstBuilder.String(), secondBuilder.String())
}

func TestBuildLeafGroupKey_UsesCachedKey(t *testing.T) {
	nodes := createSpanNodesWithDurations(t, []int64{100})
	node := nodes[0]

	node.groupKey = "cached-group-key"

	processor := &spanPruningProcessor{}

	result := processor.buildLeafGroupKey(node)

	assert.Equal(t, "cached-group-key", result)
}

func TestBuildLeafGroupKey_DifferentDepthsProduceDifferentKeys(t *testing.T) {
	nodes := createSpanNodesWithDurations(t, []int64{100, 200})

	parent := nodes[0]
	child := nodes[1]

	parent.span.SetName("handler")
	child.span.SetName("handler")

	child.parent = parent

	processor := &spanPruningProcessor{}

	parentKey := processor.buildLeafGroupKey(parent)
	childKey := processor.buildLeafGroupKey(child)

	assert.NotEqual(t, parentKey, childKey)
}

func TestBuildParentGroupKey_DifferentDepthsProduceDifferentKeys(t *testing.T) {
	nodes := createSpanNodesWithDurations(t, []int64{100, 200})

	nodes[0].span.SetName("handler")
	nodes[1].span.SetName("handler")

	processor := &spanPruningProcessor{}

	depthOneKey := processor.buildParentGroupKey(nodes[0].span, 1)
	depthTwoKey := processor.buildParentGroupKey(nodes[1].span, 2)

	assert.NotEqual(t, depthOneKey, depthTwoKey)
}

func TestGroupLeafNodesByKey_GroupsEquivalentNodes(t *testing.T) {
	nodes := createSpanNodesWithDurations(t, []int64{100, 200, 300})

	processor := &spanPruningProcessor{}

	groups := processor.groupLeafNodesByKey(nodes)

	require.Len(t, groups, 1)

	for _, group := range groups {
		assert.Len(t, group, 3)
	}
}

func TestGroupLeafNodesByKey_SeparatesDifferentNames(t *testing.T) {
	nodes := createSpanNodesWithDurations(t, []int64{100, 200})

	nodes[0].span.SetName("operation-a")
	nodes[1].span.SetName("operation-b")

	processor := &spanPruningProcessor{}

	groups := processor.groupLeafNodesByKey(nodes)

	require.Len(t, groups, 2)

	for _, group := range groups {
		assert.Len(t, group, 1)
	}
}
