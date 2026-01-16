// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package profiles

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlprofile"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
)

func Test_ProfileFunctions(t *testing.T) {
	expected := ottlfuncs.StandardFuncs[ottlprofile.TransformContext]()
	actual := ProfileFunctions()
	require.Len(t, expected, len(actual))
	for k := range actual {
		assert.Contains(t, expected, k)
	}
}
