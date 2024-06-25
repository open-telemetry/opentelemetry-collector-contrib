// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
)

func Test_IsRootSpan(t *testing.T) {
	exprFunc, err := isRootSpan()
	assert.NoError(t, err)

	// root span
	spanRoot := ptrace.NewSpan()
	spanRoot.SetParentSpanID(pcommon.SpanID{
		0, 0, 0, 0, 0, 0, 0, 0,
	})

	value, err := exprFunc(nil, ottlspan.NewTransformContext(spanRoot, pcommon.NewInstrumentationScope(), pcommon.NewResource(), ptrace.NewScopeSpans(), ptrace.NewResourceSpans()))
	assert.NoError(t, err)
	require.Equal(t, true, value)

	// non root span
	spanNonRoot := ptrace.NewSpan()
	spanNonRoot.SetParentSpanID(pcommon.SpanID{
		1, 0, 0, 0, 0, 0, 0, 0,
	})

	value, err = exprFunc(nil, ottlspan.NewTransformContext(spanNonRoot, pcommon.NewInstrumentationScope(), pcommon.NewResource(), ptrace.NewScopeSpans(), ptrace.NewResourceSpans()))
	assert.NoError(t, err)
	require.Equal(t, false, value)
}
