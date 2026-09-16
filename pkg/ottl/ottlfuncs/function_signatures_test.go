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

const signaturesGolden = "testdata/function_signatures.txt"

const ottlPkgPath = "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"

// Test_FunctionSignatures locks the OTTL function surface: name, argument
// order, argument type, and optionality. Any diff here is a breaking change to
// the statements users write, so it must be paired with a changelog entry and,
// once the signals are stable, treated as a compatibility break.
//
// Regenerate intentionally from the pkg/ottl module with:
//
//	make update-ottl-signatures
func Test_FunctionSignatures(t *testing.T) {
	got := renderSignatures(StandardFuncs[any]())

	if os.Getenv("OTTL_UPDATE_SIGNATURES") != "" {
		require.NoError(t, os.MkdirAll(filepath.Dir(signaturesGolden), 0o750))
		require.NoError(t, os.WriteFile(signaturesGolden, []byte(got), 0o600))
		return
	}

	want, err := os.ReadFile(signaturesGolden)
	require.NoError(t, err, "missing golden; run: make update-ottl-signatures")
	require.Equal(t, string(want), got,
		"OTTL function signatures changed — this is a BREAKING change to the OTTL language surface. "+
			"If intended, regenerate the golden (make update-ottl-signatures) and add a changelog entry.")
}

func renderSignatures(funcs map[string]ottl.Factory[any]) string {
	names := make([]string, 0, len(funcs))
	for name := range funcs {
		names = append(names, name)
	}
	slices.Sort(names)

	var b strings.Builder
	for _, name := range names {
		fmt.Fprintln(&b, renderSignature(funcs[name]))
	}
	return b.String()
}

func renderSignature(f ottl.Factory[any]) string {
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
