// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package expotest // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/expo/expotest"

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/expo"
)

type I struct {
	t *testing.T
}

func Is(t *testing.T) *I {
	return &I{t: t}
}

func (is *I) Equal(want, got any) {
	is.t.Helper()
	equal(is.t, want, got, "")
}

func (is *I) Equalf(want, got any, name string) {
	is.t.Helper()
	equal(is.t, want, got, name)
}

func equal(t *testing.T, want, got any, name string) bool {
	t.Helper()
	require.IsType(t, want, got)

	vw := reflect.ValueOf(want)
	vg := reflect.ValueOf(got)

	if vw.Kind() != reflect.Struct {
		ok := reflect.DeepEqual(want, got)
		if !ok {
			t.Errorf("%s: %+v != %+v", name, want, got)
		}
		return ok
	}

	ok := true
	// compare all "getters" of the struct
	for i := 0; i < vw.NumMethod(); i++ {
		mname := vw.Type().Method(i).Name
		fname := strings.TrimPrefix(name+"."+mname+"()", ".")

		mw := vw.Method(i)
		mg := vg.Method(i)

		// only compare "getters"
		if mw.Type().NumIn() != 0 || mw.Type().NumOut() != 1 {
			continue
		}
		// Append(Empty) fails above heuristic, exclude it
		if strings.HasPrefix(mname, "Append") {
			continue
		}

		rw := mw.Call(nil)[0].Interface()
		rg := mg.Call(nil)[0].Interface()

		ok = equal(t, rw, rg, fname) && ok
	}

	// compare all exported fields of the struct
	for i := 0; i < vw.NumField(); i++ {
		if !vw.Type().Field(i).IsExported() {
			continue
		}
		fname := name + "." + vw.Type().Field(i).Name
		fw := vw.Field(i).Interface()
		fg := vg.Field(i).Interface()
		ok = equal(t, fw, fg, fname) && ok
	}
	if !ok {
		return false
	}

	if _, ok := want.(expo.DataPoint); ok {
		err := pmetrictest.CompareExponentialHistogramDataPoint(want.(expo.DataPoint), got.(expo.DataPoint))
		if err != nil {
			t.Error(err)
		}
	}

	// fallback to a full deep-equal for rare cases (unexported fields, etc)
	return assert.Equal(t, want, got)
}
