// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func assertArgumentFieldNames(t *testing.T, args ottl.Arguments, expected []string) {
	t.Helper()
	typ := reflect.TypeOf(args)
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	require.Equal(t, reflect.Struct, typ.Kind())
	got := make([]string, 0, typ.NumField())
	for i := 0; i < typ.NumField(); i++ {
		got = append(got, typ.Field(i).Name)
	}
	assert.Equal(t, expected, got)
}
