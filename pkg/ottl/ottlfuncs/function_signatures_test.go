// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

const (
	stableSignaturesGolden       = "testdata/stable_function_signatures.txt"
	experimentalSignaturesGolden = "testdata/experimental_function_signatures.txt"
)

const ottlPkgPath = "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"

// Test_StableFunctionSignatures locks the stable OTTL function surface: name,
// argument order, argument type, and optionality. Any diff here is a breaking
// change to the statements users write, so it must be paired with a changelog
// entry and treated as a compatibility break. A function moving between the
// stable and experimental goldens is itself a surface change.
//
// Regenerate intentionally from the pkg/ottl module with:
//
//	make update-ottl-signatures
func Test_StableFunctionSignatures(t *testing.T) {
	assertSignaturesGolden(t, stableSignaturesGolden, renderSignatures(false))
}

// Test_ExperimentalFunctionSignatures locks the experimental OTTL function
// surface. Experimental functions are not covered by the 1.0 stability
// guarantee, but the golden still makes any change to their surface, and any
// promotion to the stable golden, an intentional reviewed diff.
//
// Regenerate intentionally from the pkg/ottl module with:
//
//	make update-ottl-signatures
func Test_ExperimentalFunctionSignatures(t *testing.T) {
	assertSignaturesGolden(t, experimentalSignaturesGolden, renderSignatures(true))
}

func assertSignaturesGolden(t *testing.T, golden, got string) {
	t.Helper()
	if os.Getenv("OTTL_UPDATE_SIGNATURES") != "" {
		require.NoError(t, os.MkdirAll(filepath.Dir(golden), 0o750))
		require.NoError(t, os.WriteFile(golden, []byte(got), 0o600))
		return
	}

	want, err := os.ReadFile(golden)
	require.NoError(t, err, "missing golden; run: make update-ottl-signatures")
	require.Equal(t, string(want), got,
		"OTTL function signatures changed — this is a change to the OTTL language surface. "+
			"If intended, regenerate the goldens (make update-ottl-signatures) and add a changelog entry.")
}

func renderSignatures(experimental bool) string {
	var lines []string
	for _, f := range StandardFuncs[any]() {
		if f.Experimental() == experimental {
			lines = append(lines, renderSignature(f))
		}
	}

	if !experimental {
		lines = append(lines, renderSignature(NewIsRootSpanFactory()))
	}
	slices.Sort(lines)

	var b strings.Builder
	for _, line := range lines {
		fmt.Fprintln(&b, line)
	}
	return b.String()
}

func renderSignature[K any](f ottl.Factory[K]) string {
	args := f.CreateDefaultArguments()
	if args == nil {
		return f.Name() + "()"
	}
	typ := reflect.TypeOf(args)
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}

	fields := make([]string, 0, typ.NumField())
	for field := range typ.Fields() {
		fieldType := strings.ReplaceAll(field.Type.String(), ottlPkgPath+".", "ottl.")
		fields = append(fields, field.Name+" "+fieldType)
	}
	return fmt.Sprintf("%s(%s)", f.Name(), strings.Join(fields, ", "))
}
