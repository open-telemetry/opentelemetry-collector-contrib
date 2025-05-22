// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_LogFunctions(t *testing.T) {
	expected := ottlfuncs.StandardFuncs[ottllog.TransformContext]()
	actual := LogFunctions()
	require.Len(t, actual, len(expected))
	for k := range actual {
		assert.Contains(t, expected, k)
	}
}
