// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
)

func Test_GetSliceValue_Invalid(t *testing.T) {
	tests := []struct {
		name string
		keys func() ottl.Key
		err  error
	}{
		{
			name: "no keys",
			keys: func() ottl.Key { return ottl.NewEmptyKey() },
			err:  fmt.Errorf("cannot get slice value without key"),
		},
		{
			name: "first key not an integer",
			keys: func() ottl.Key {
				k := ottl.NewEmptyKey()
				k.SetString(ottltest.Strp("key"))
				return k
			},
			err: fmt.Errorf("non-integer indexing is not supported"),
		},
		{
			name: "index too large",
			keys: func() ottl.Key {
				k := ottl.NewEmptyKey()
				k.SetInt(ottltest.Intp(1))
				return k
			},
			err: fmt.Errorf("index 1 out of bounds"),
		},
		{
			name: "index too small",
			keys: func() ottl.Key {
				k := ottl.NewEmptyKey()
				k.SetInt(ottltest.Intp(-1))
				return k
			},
			err: fmt.Errorf("index -1 out of bounds"),
		},
		{
			name: "invalid type",
			keys: func() ottl.Key {
				k := ottl.NewEmptyKey()
				k.SetInt(ottltest.Intp(0))
				k2 := ottl.NewEmptyKey()
				k2.SetString(ottltest.Strp("string"))
				k.SetNext(&k2)
				return k
			},
			err: fmt.Errorf("type Str does not support string indexing"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := pcommon.NewSlice()
			s.AppendEmpty().SetStr("val")

			_, err := GetSliceValue(s, tt.keys())
			assert.Equal(t, tt.err, err)
		})
	}
}

func Test_SetSliceValue_Invalid(t *testing.T) {
	tests := []struct {
		name string
		keys func() ottl.Key
		err  error
	}{
		{
			name: "no keys",
			keys: func() ottl.Key { return ottl.NewEmptyKey() },
			err:  fmt.Errorf("cannot set slice value without key"),
		},
		{
			name: "first key not an integer",
			keys: func() ottl.Key {
				k := ottl.NewEmptyKey()
				k.SetString(ottltest.Strp("key"))
				return k
			},
			err: fmt.Errorf("non-integer indexing is not supported"),
		},
		{
			name: "index too large",
			keys: func() ottl.Key {
				k := ottl.NewEmptyKey()
				k.SetInt(ottltest.Intp(1))
				return k
			},
			err: fmt.Errorf("index 1 out of bounds"),
		},
		{
			name: "index too small",
			keys: func() ottl.Key {
				k := ottl.NewEmptyKey()
				k.SetInt(ottltest.Intp(-1))
				return k
			},
			err: fmt.Errorf("index -1 out of bounds"),
		},
		{
			name: "invalid type",
			keys: func() ottl.Key {
				k := ottl.NewEmptyKey()
				k.SetInt(ottltest.Intp(0))
				k2 := ottl.NewEmptyKey()
				k2.SetString(ottltest.Strp("string"))
				k.SetNext(&k2)
				return k
			},
			err: fmt.Errorf("type Str does not support string indexing"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := pcommon.NewSlice()
			s.AppendEmpty().SetStr("val")

			err := SetSliceValue(s, tt.keys(), "value")
			assert.Equal(t, tt.err, err)
		})
	}
}
