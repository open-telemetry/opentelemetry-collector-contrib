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

const experimentalSignaturesGolden = "testdata/experimental_function_signatures.txt"

const ottlPkgPath = "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"

// Test_ExperimentalFunctionSignatures locks the experimental OTTL function
// surface. Experimental functions are not covered by OTTL's stability
// guarantee, but the golden still makes any change to their surface, and any
// promotion to the stable golden, an intentional reviewed diff.
//
// Regenerate intentionally from the pkg/ottl/xottl module with:
//
//	make update-ottl-signatures
func Test_ExperimentalFunctionSignatures(t *testing.T) {
	var lines []string
	for _, f := range ExperimentalConverters[any]() {
		lines = append(lines, renderSignature(f))
	}
	slices.Sort(lines)

	var b strings.Builder
	for _, line := range lines {
		fmt.Fprintln(&b, line)
	}
	got := b.String()

	if os.Getenv("OTTL_UPDATE_SIGNATURES") != "" {
		require.NoError(t, os.MkdirAll(filepath.Dir(experimentalSignaturesGolden), 0o750))
		require.NoError(t, os.WriteFile(experimentalSignaturesGolden, []byte(got), 0o600))
		return
	}

	want, err := os.ReadFile(experimentalSignaturesGolden)
	require.NoError(t, err, "missing golden; run: make update-ottl-signatures")
	require.Equal(t, string(want), got,
		"OTTL function signatures changed — this is a change to the OTTL language surface. "+
			"If intended, regenerate the golden (make update-ottl-signatures) and add a changelog entry.")
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
