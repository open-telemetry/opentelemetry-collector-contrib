// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
)

// TestFrozenStandardFunctions guards the OTTL function surface against accidental
// changes.
//
// With the ottl.functions.enableLambda and ottl.functions.enableExperimental feature
// gates disabled, StandardFuncs returns only the frozen standard functions. That set
// is covered by OTTL's stability guarantee and must not change within a major
// version: any add or remove fails the "gates disabled" case on purpose. The change
// is only allowed by regenerating the golden file (OTTL_UPDATE_GOLDEN=1 go test
// ./...), which produces a reviewable diff that the OTTL code owners must approve.
//
// With both gates enabled, StandardFuncs additionally returns the lambda and
// experimental functions. Those sets are not frozen, so this golden file is expected
// to change as such functions are added, promoted, or removed.
func TestFrozenStandardFunctions(t *testing.T) {
	t.Run("gates disabled", func(t *testing.T) {
		defer ottltest.SetFeatureGateForTest(t, metadata.OttlFunctionsEnableLambdaFeatureGate, false)()
		defer ottltest.SetFeatureGateForTest(t, metadata.OttlFunctionsEnableExperimentalFeatureGate, false)()
		checkGolden(t, filepath.Join("testdata", "functions_gates_disabled.txt"), sortedNames(StandardFuncs[any]()))
	})

	t.Run("gates enabled", func(t *testing.T) {
		defer ottltest.SetFeatureGateForTest(t, metadata.OttlFunctionsEnableLambdaFeatureGate, true)()
		defer ottltest.SetFeatureGateForTest(t, metadata.OttlFunctionsEnableExperimentalFeatureGate, true)()
		checkGolden(t, filepath.Join("testdata", "functions_gates_enabled.txt"), sortedNames(StandardFuncs[any]()))
	})
}

func sortedNames(m map[string]ottl.Factory[any]) []string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func checkGolden(t *testing.T, path string, got []string) {
	t.Helper()
	want := strings.Join(got, "\n") + "\n"

	if os.Getenv("OTTL_UPDATE_GOLDEN") != "" {
		require.NoError(t, os.WriteFile(path, []byte(want), 0o600))
		return
	}

	data, err := os.ReadFile(path)
	require.NoError(t, err, "missing golden file %s; regenerate with OTTL_UPDATE_GOLDEN=1", path)
	assert.Equal(t, string(data), want, "%s is out of date; if this change is intentional regenerate with OTTL_UPDATE_GOLDEN=1 and get OTTL code owner approval", path)
}
