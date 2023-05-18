// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package correlation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestSpanShimList(t *testing.T) {
	spans := ptrace.NewResourceSpansSlice()
	spans.EnsureCapacity(2)
	s1 := spans.AppendEmpty()
	s2 := spans.AppendEmpty()
	wrapped := spanListWrap{spans}
	assert.Equal(t, 2, wrapped.Len())
	assert.Equal(t, spanWrap{s1}, wrapped.At(0))
	assert.Equal(t, spanWrap{s2}, wrapped.At(1))
}

func TestSpanShimList_Empty(t *testing.T) {
	spans := ptrace.NewResourceSpansSlice()
	wrapped := spanListWrap{spans}
	assert.Equal(t, 0, wrapped.Len())
}

func TestSpanShim_Service(t *testing.T) {
	span := ptrace.NewResourceSpans()
	res := span.Resource()
	attr := res.Attributes()
	attr.PutStr("service.name", "shopping-cart")

	wrapped := spanWrap{span}

	service, ok := wrapped.ServiceName()
	require.True(t, ok)

	assert.Equal(t, "shopping-cart", service)
}

func TestSpanShim_Environment(t *testing.T) {
	span := ptrace.NewResourceSpans()
	res := span.Resource()
	attr := res.Attributes()
	attr.PutStr("deployment.environment", "prod")

	wrapped := spanWrap{span}

	environment, ok := wrapped.Environment()
	require.True(t, ok)

	assert.Equal(t, "prod", environment)
}

func TestSpanShim_SignalfxEnvironment(t *testing.T) {
	span := ptrace.NewResourceSpans()
	res := span.Resource()
	attr := res.Attributes()
	attr.PutStr("environment", "prod")

	wrapped := spanWrap{span}

	environment, ok := wrapped.Environment()
	require.True(t, ok)

	assert.Equal(t, "prod", environment)
}

func TestSpanShim_Missing(t *testing.T) {
	span := ptrace.NewResourceSpans()
	wrapped := spanWrap{span}

	_, ok := wrapped.Environment()
	assert.False(t, ok)
	_, ok = wrapped.ServiceName()
	assert.False(t, ok)
}

func TestSpanShim_ResourceNil(t *testing.T) {
	span := ptrace.NewResourceSpans()

	wrapped := spanWrap{span}

	_, ok := wrapped.Environment()
	assert.False(t, ok)
	_, ok = wrapped.ServiceName()
	assert.False(t, ok)
	_, ok = wrapped.Tag("tag")
	assert.False(t, ok)

	assert.Equal(t, 0, wrapped.NumTags())
}

func TestSpanShim_Tags(t *testing.T) {
	span := ptrace.NewResourceSpans()
	res := span.Resource()
	attr := res.Attributes()
	attr.PutStr("tag1", "tag1val")

	wrapped := spanWrap{span}

	assert.Equal(t, 1, wrapped.NumTags())

	tag, ok := wrapped.Tag("tag1")
	assert.True(t, ok)
	assert.Equal(t, "tag1val", tag)

	tag, ok = wrapped.Tag("missing")
	assert.False(t, ok)
	assert.Equal(t, "", tag)
}
