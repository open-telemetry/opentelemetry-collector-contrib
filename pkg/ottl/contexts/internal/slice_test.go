// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"context"
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
		keys []ottl.Key[any]
		err  error
	}{
		{
			name: "first key not an integer",
			keys: []ottl.Key[any]{
				&TestKey[any]{
					S: ottltest.Strp("key"),
				},
			},
			err: fmt.Errorf("non-integer indexing is not supported"),
		},
		{
			name: "index too large",
			keys: []ottl.Key[any]{
				&TestKey[any]{
					I: ottltest.Intp(1),
				},
			},
			err: fmt.Errorf("index 1 out of bounds"),
		},
		{
			name: "index too small",
			keys: []ottl.Key[any]{
				&TestKey[any]{
					I: ottltest.Intp(-1),
				},
			},
			err: fmt.Errorf("index -1 out of bounds"),
		},
		{
			name: "invalid type",
			keys: []ottl.Key[any]{
				&TestKey[any]{
					I: ottltest.Intp(0),
				},
				&TestKey[any]{
					S: ottltest.Strp("string"),
				},
			},
			err: fmt.Errorf("type Str does not support string indexing"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := pcommon.NewSlice()
			s.AppendEmpty().SetStr("val")

			_, err := GetSliceValue[any](context.Background(), nil, s, tt.keys)
			assert.Equal(t, tt.err, err)
		})
	}
}

func Test_GetSliceValue_NilKey(t *testing.T) {
	_, err := GetSliceValue[any](context.Background(), nil, pcommon.NewSlice(), nil)
	assert.Error(t, err)
}

func Test_SetSliceValue_Invalid(t *testing.T) {
	tests := []struct {
		name string
		keys []ottl.Key[any]
		err  error
	}{
		{
			name: "first key not an integer",
			keys: []ottl.Key[any]{
				&TestKey[any]{
					S: ottltest.Strp("key"),
				},
			},
			err: fmt.Errorf("non-integer indexing is not supported"),
		},
		{
			name: "index too large",
			keys: []ottl.Key[any]{
				&TestKey[any]{
					I: ottltest.Intp(1),
				},
			},
			err: fmt.Errorf("index 1 out of bounds"),
		},
		{
			name: "index too small",
			keys: []ottl.Key[any]{
				&TestKey[any]{
					I: ottltest.Intp(-1),
				},
			},
			err: fmt.Errorf("index -1 out of bounds"),
		},
		{
			name: "invalid type",
			keys: []ottl.Key[any]{
				&TestKey[any]{
					I: ottltest.Intp(0),
				},
				&TestKey[any]{
					S: ottltest.Strp("string"),
				},
			},
			err: fmt.Errorf("type Str does not support string indexing"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := pcommon.NewSlice()
			s.AppendEmpty().SetStr("val")

			err := SetSliceValue[any](context.Background(), nil, s, tt.keys, "value")
			assert.Equal(t, tt.err, err)
		})
	}
}

func Test_SetSliceValue_NilKey(t *testing.T) {
	err := SetSliceValue[any](context.Background(), nil, pcommon.NewSlice(), nil, "value")
	assert.Error(t, err)
}
