// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/logparsingfuncs"
)

func Test_LogFunctions(t *testing.T) {
	expected := ottlfuncs.StandardFuncs[*ottllog.TransformContext]()
	expected["ParseCLF"] = logparsingfuncs.NewParseCLFFactory()
	expected["ParseLEEF"] = logparsingfuncs.NewParseLEEFFactory()
	expected["ParseELF"] = logparsingfuncs.NewParseELFFactory()
	actual := LogFunctions()
	require.Len(t, actual, len(expected))
	for k := range actual {
		assert.Contains(t, expected, k)
	}
}
