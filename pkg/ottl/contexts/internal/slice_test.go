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
		keys []ottl.key
		err  error
	}{
		{
			name: "no keys",
			keys: []ottl.key{},
			err:  fmt.Errorf("cannot get slice value without key"),
		},
		{
			name: "first key not an integer",
			keys: []ottl.key{
				{
					String: ottltest.Strp("key"),
				},
			},
			err: fmt.Errorf("non-integer indexing is not supported"),
		},
		{
			name: "index too large",
			keys: []ottl.key{
				{
					Int: ottltest.Intp(1),
				},
			},
			err: fmt.Errorf("index 1 out of bounds"),
		},
		{
			name: "index too small",
			keys: []ottl.key{
				{
					Int: ottltest.Intp(-1),
				},
			},
			err: fmt.Errorf("index -1 out of bounds"),
		},
		{
			name: "invalid type",
			keys: []ottl.key{
				{
					Int: ottltest.Intp(0),
				},
				{
					String: ottltest.Strp("string"),
				},
			},
			err: fmt.Errorf("type Str does not support string indexing"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := pcommon.NewSlice()
			s.AppendEmpty().SetStr("val")

			_, err := GetSliceValue(s, tt.keys)
			assert.Equal(t, tt.err, err)
		})
	}
}

func Test_SetSliceValue_Invalid(t *testing.T) {
	tests := []struct {
		name string
		keys []ottl.key
		err  error
	}{
		{
			name: "no keys",
			keys: []ottl.key{},
			err:  fmt.Errorf("cannot set slice value without key"),
		},
		{
			name: "first key not an integer",
			keys: []ottl.key{
				{
					String: ottltest.Strp("key"),
				},
			},
			err: fmt.Errorf("non-integer indexing is not supported"),
		},
		{
			name: "index too large",
			keys: []ottl.key{
				{
					Int: ottltest.Intp(1),
				},
			},
			err: fmt.Errorf("index 1 out of bounds"),
		},
		{
			name: "index too small",
			keys: []ottl.key{
				{
					Int: ottltest.Intp(-1),
				},
			},
			err: fmt.Errorf("index -1 out of bounds"),
		},
		{
			name: "invalid type",
			keys: []ottl.key{
				{
					Int: ottltest.Intp(0),
				},
				{
					String: ottltest.Strp("string"),
				},
			},
			err: fmt.Errorf("type Str does not support string indexing"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := pcommon.NewSlice()
			s.AppendEmpty().SetStr("val")

			err := SetSliceValue(s, tt.keys, "value")
			assert.Equal(t, tt.err, err)
		})
	}
}
