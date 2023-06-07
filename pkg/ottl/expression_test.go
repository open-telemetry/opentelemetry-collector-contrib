// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
)

func hello() (ExprFunc[any], error) {
	return func(ctx context.Context, tCtx any) (any, error) {
		return "world", nil
	}, nil
}

func pmap() (ExprFunc[any], error) {
	return func(ctx context.Context, tCtx any) (interface{}, error) {
		m := pcommon.NewMap()
		m.PutEmptyMap("foo").PutStr("bar", "pass")
		return m, nil
	}, nil
}

func basicMap() (ExprFunc[any], error) {
	return func(ctx context.Context, tCtx any) (interface{}, error) {
		return map[string]interface{}{
			"foo": map[string]interface{}{
				"bar": "pass",
			},
		}, nil
	}, nil
}

func pslice() (ExprFunc[any], error) {
	return func(ctx context.Context, tCtx any) (interface{}, error) {
		s := pcommon.NewSlice()
		s.AppendEmpty().SetEmptySlice().AppendEmpty().SetStr("pass")
		return s, nil
	}, nil
}

func basicSlice() (ExprFunc[any], error) {
	return func(ctx context.Context, tCtx any) (interface{}, error) {
		return []interface{}{
			[]interface{}{
				"pass",
			},
		}, nil
	}, nil
}

func Test_newGetter(t *testing.T) {
	tests := []struct {
		name string
		val  value
		ctx  interface{}
		want interface{}
	}{
		{
			name: "string literal",
			val: value{
				String: ottltest.Strp("str"),
			},
			want: "str",
		},
		{
			name: "float literal",
			val: value{
				Literal: &mathExprLiteral{
					Float: ottltest.Floatp(1.2),
				},
			},
			want: 1.2,
		},
		{
			name: "int literal",
			val: value{
				Literal: &mathExprLiteral{
					Int: ottltest.Intp(12),
				},
			},
			want: int64(12),
		},
		{
			name: "bytes literal",
			val: value{
				Bytes: (*byteSlice)(&[]byte{1, 2, 3, 4, 5, 6, 7, 8}),
			},
			want: []byte{1, 2, 3, 4, 5, 6, 7, 8},
		},
		{
			name: "nil literal",
			val: value{
				IsNil: (*isNil)(ottltest.Boolp(true)),
			},
			want: nil,
		},
		{
			name: "bool literal",
			val: value{
				Bool: (*boolean)(ottltest.Boolp(true)),
			},
			want: true,
		},
		{
			name: "path expression",
			val: value{
				Literal: &mathExprLiteral{
					Path: &path{
						Fields: []field{
							{
								Name: "name",
							},
						},
					},
				},
			},
			want: "bear",
		},
		{
			name: "complex path expression",
			val: value{
				Literal: &mathExprLiteral{
					Path: &path{
						Fields: []field{
							{
								Name: "attributes",
								Keys: []key{
									{
										String: ottltest.Strp("foo"),
									},
									{
										String: ottltest.Strp("bar"),
									},
								},
							},
						},
					},
				},
			},
			want: "pass",
		},
		{
			name: "function call",
			val: value{
				Literal: &mathExprLiteral{
					Converter: &converter{
						Function: "Hello",
					},
				},
			},
			want: "world",
		},
		{
			name: "function call nested pcommon map",
			val: value{
				Literal: &mathExprLiteral{
					Converter: &converter{
						Function: "PMap",
						Keys: []key{
							{
								String: ottltest.Strp("foo"),
							},
							{
								String: ottltest.Strp("bar"),
							},
						},
					},
				},
			},
			want: "pass",
		},
		{
			name: "function call nested map",
			val: value{
				Literal: &mathExprLiteral{
					Converter: &converter{
						Function: "Map",
						Keys: []key{
							{
								String: ottltest.Strp("foo"),
							},
							{
								String: ottltest.Strp("bar"),
							},
						},
					},
				},
			},
			want: "pass",
		},
		{
			name: "function call pcommon slice",
			val: value{
				Literal: &mathExprLiteral{
					Converter: &converter{
						Function: "PSlice",
						Keys: []key{
							{
								Int: ottltest.Intp(0),
							},
							{
								Int: ottltest.Intp(0),
							},
						},
					},
				},
			},
			want: "pass",
		},
		{
			name: "function call nested slice",
			val: value{
				Literal: &mathExprLiteral{
					Converter: &converter{
						Function: "Slice",
						Keys: []key{
							{
								Int: ottltest.Intp(0),
							},
							{
								Int: ottltest.Intp(0),
							},
						},
					},
				},
			},
			want: "pass",
		},
		{
			name: "enum",
			val: value{
				Enum: (*enumSymbol)(ottltest.Strp("TEST_ENUM_ONE")),
			},
			want: int64(1),
		},
		{
			name: "empty list",
			val: value{
				List: &list{
					Values: []value{},
				},
			},
			want: []any{},
		},
		{
			name: "string list",
			val: value{
				List: &list{
					Values: []value{
						{
							String: ottltest.Strp("test0"),
						},
						{
							String: ottltest.Strp("test1"),
						},
					},
				},
			},
			want: []any{"test0", "test1"},
		},
		{
			name: "int list",
			val: value{
				List: &list{
					Values: []value{
						{
							Literal: &mathExprLiteral{
								Int: ottltest.Intp(1),
							},
						},
						{
							Literal: &mathExprLiteral{
								Int: ottltest.Intp(2),
							},
						},
					},
				},
			},
			want: []any{int64(1), int64(2)},
		},
		{
			name: "float list",
			val: value{
				List: &list{
					Values: []value{
						{
							Literal: &mathExprLiteral{
								Float: ottltest.Floatp(1.2),
							},
						},
						{
							Literal: &mathExprLiteral{
								Float: ottltest.Floatp(2.4),
							},
						},
					},
				},
			},
			want: []any{1.2, 2.4},
		},
		{
			name: "bool list",
			val: value{
				List: &list{
					Values: []value{
						{
							Bool: (*boolean)(ottltest.Boolp(true)),
						},
						{
							Bool: (*boolean)(ottltest.Boolp(false)),
						},
					},
				},
			},
			want: []any{true, false},
		},
		{
			name: "byte slice list",
			val: value{
				List: &list{
					Values: []value{
						{
							Bytes: (*byteSlice)(&[]byte{1, 2, 3, 4, 5, 6, 7, 8}),
						},
						{
							Bytes: (*byteSlice)(&[]byte{9, 8, 7, 6, 5, 4, 3, 2}),
						},
					},
				},
			},
			want: []any{[]byte{1, 2, 3, 4, 5, 6, 7, 8}, []byte{9, 8, 7, 6, 5, 4, 3, 2}},
		},
		{
			name: "path expression",
			val: value{
				List: &list{
					Values: []value{
						{
							Literal: &mathExprLiteral{
								Path: &path{
									Fields: []field{
										{
											Name: "name",
										},
									},
								},
							},
						},
					},
				},
			},
			ctx:  "bear",
			want: []any{"bear"},
		},
		{
			name: "function call",
			val: value{
				List: &list{
					Values: []value{
						{
							Literal: &mathExprLiteral{
								Converter: &converter{
									Function: "Hello",
								},
							},
						},
					},
				},
			},
			want: []any{"world"},
		},
		{
			name: "nil slice",
			val: value{
				List: &list{
					Values: []value{
						{
							IsNil: (*isNil)(ottltest.Boolp(true)),
						},
						{
							IsNil: (*isNil)(ottltest.Boolp(true)),
						},
					},
				},
			},
			want: []any{nil, nil},
		},
		{
			name: "heterogeneous slice",
			val: value{
				List: &list{
					Values: []value{
						{
							String: ottltest.Strp("test0"),
						},
						{
							Literal: &mathExprLiteral{
								Int: ottltest.Intp(1),
							},
						},
					},
				},
			},
			want: []any{"test0", int64(1)},
		},
	}

	functions := CreateFactoryMap(
		createFactory("Hello", &struct{}{}, hello),
		createFactory("PMap", &struct{}{}, pmap),
		createFactory("Map", &struct{}{}, basicMap),
		createFactory("PSlice", &struct{}{}, pslice),
		createFactory("Slice", &struct{}{}, basicSlice),
	)

	p, _ := NewParser[any](
		functions,
		testParsePath,
		componenttest.NewNopTelemetrySettings(),
		WithEnumParser[any](testParseEnum),
	)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader, err := p.newGetter(tt.val)
			assert.NoError(t, err)

			tCtx := tt.want

			if tt.ctx != nil {
				tCtx = tt.ctx
			}

			val, err := reader.Get(context.Background(), tCtx)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, val)
		})
	}

	t.Run("empty value", func(t *testing.T) {
		_, err := p.newGetter(value{})
		assert.Error(t, err)
	})
}
func Test_exprGetter_Get_Invalid(t *testing.T) {
	tests := []struct {
		name string
		val  value
		err  error
	}{
		{
			name: "key not in pcommon map",
			val: value{
				Literal: &mathExprLiteral{
					Converter: &converter{
						Function: "PMap",
						Keys: []key{
							{
								String: ottltest.Strp("unknown key"),
							},
						},
					},
				},
			},
			err: fmt.Errorf("key not found in map"),
		},
		{
			name: "key not in map",
			val: value{
				Literal: &mathExprLiteral{
					Converter: &converter{
						Function: "Map",
						Keys: []key{
							{
								String: ottltest.Strp("unknown key"),
							},
						},
					},
				},
			},
			err: fmt.Errorf("key not found in map"),
		},
		{
			name: "index too large for pcommon slice",
			val: value{
				Literal: &mathExprLiteral{
					Converter: &converter{
						Function: "PSlice",
						Keys: []key{
							{
								Int: ottltest.Intp(100),
							},
						},
					},
				},
			},
			err: fmt.Errorf("index 100 out of bounds"),
		},
		{
			name: "negative for pcommon slice",
			val: value{
				Literal: &mathExprLiteral{
					Converter: &converter{
						Function: "PSlice",
						Keys: []key{
							{
								Int: ottltest.Intp(-1),
							},
						},
					},
				},
			},
			err: fmt.Errorf("index -1 out of bounds"),
		},
		{
			name: "index too large for Go slice",
			val: value{
				Literal: &mathExprLiteral{
					Converter: &converter{
						Function: "Slice",
						Keys: []key{
							{
								Int: ottltest.Intp(100),
							},
						},
					},
				},
			},
			err: fmt.Errorf("index 100 out of bounds"),
		},
		{
			name: "negative for Go slice",
			val: value{
				Literal: &mathExprLiteral{
					Converter: &converter{
						Function: "Slice",
						Keys: []key{
							{
								Int: ottltest.Intp(-1),
							},
						},
					},
				},
			},
			err: fmt.Errorf("index -1 out of bounds"),
		},
		{
			name: "invalid int indexing type",
			val: value{
				Literal: &mathExprLiteral{
					Converter: &converter{
						Function: "Hello",
						Keys: []key{
							{
								Int: ottltest.Intp(-1),
							},
						},
					},
				},
			},
			err: fmt.Errorf("type, string, does not support int indexing"),
		},
		{
			name: "invalid string indexing type",
			val: value{
				Literal: &mathExprLiteral{
					Converter: &converter{
						Function: "Hello",
						Keys: []key{
							{
								String: ottltest.Strp("test"),
							},
						},
					},
				},
			},
			err: fmt.Errorf("type, string, does not support string indexing"),
		},
	}

	functions := CreateFactoryMap(
		createFactory("Hello", &struct{}{}, hello),
		createFactory("PMap", &struct{}{}, pmap),
		createFactory("Map", &struct{}{}, basicMap),
		createFactory("PSlice", &struct{}{}, pslice),
		createFactory("Slice", &struct{}{}, basicSlice),
	)

	p, _ := NewParser[any](
		functions,
		testParsePath,
		componenttest.NewNopTelemetrySettings(),
		WithEnumParser[any](testParseEnum),
	)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader, err := p.newGetter(tt.val)
			assert.NoError(t, err)
			_, err = reader.Get(context.Background(), nil)
			assert.Equal(t, tt.err, err)
		})
	}
}

func Test_StandardStringGetter(t *testing.T) {
	tests := []struct {
		name             string
		getter           StandardStringGetter[interface{}]
		want             interface{}
		valid            bool
		expectedErrorMsg string
	}{
		{
			name: "string type",
			getter: StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "str", nil
				},
			},
			want:  "str",
			valid: true,
		},
		{
			name: "ValueTypeString type",
			getter: StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return pcommon.NewValueStr("str"), nil
				},
			},
			want:  "str",
			valid: true,
		},
		{
			name: "Incorrect type",
			getter: StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return true, nil
				},
			},
			valid:            false,
			expectedErrorMsg: "expected string but got bool",
		},
		{
			name: "nil",
			getter: StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return nil, nil
				},
			},
			valid:            false,
			expectedErrorMsg: "expected string but got nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := tt.getter.Get(context.Background(), nil)
			if tt.valid {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, val)
			} else {
				assert.EqualError(t, err, tt.expectedErrorMsg)
			}
		})
	}
}

func Test_StandardStringLikeGetter(t *testing.T) {
	tests := []struct {
		name             string
		getter           StringLikeGetter[interface{}]
		want             interface{}
		valid            bool
		expectedErrorMsg string
	}{
		{
			name: "string type",
			getter: StandardStringLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "str", nil
				},
			},
			want:  "str",
			valid: true,
		},
		{
			name: "bool type",
			getter: StandardStringLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return true, nil
				},
			},
			want:  "true",
			valid: true,
		},
		{
			name: "int64 type",
			getter: StandardStringLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return int64(1), nil
				},
			},
			want:  "1",
			valid: true,
		},
		{
			name: "float64 type",
			getter: StandardStringLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return 1.1, nil
				},
			},
			want:  "1.1",
			valid: true,
		},
		{
			name: "byte[] type",
			getter: StandardStringLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return []byte{0}, nil
				},
			},
			want:  "00",
			valid: true,
		},
		{
			name: "pcommon.map type",
			getter: StandardStringLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					m := pcommon.NewMap()
					m.PutStr("test", "passed")
					return m, nil
				},
			},
			want:  `{"test":"passed"}`,
			valid: true,
		},
		{
			name: "pcommon.slice type",
			getter: StandardStringLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					s := pcommon.NewSlice()
					v := s.AppendEmpty()
					v.SetStr("test")
					return s, nil
				},
			},
			want:  `["test"]`,
			valid: true,
		},
		{
			name: "pcommon.value type",
			getter: StandardStringLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					v := pcommon.NewValueInt(int64(100))
					return v, nil
				},
			},
			want:  "100",
			valid: true,
		},
		{
			name: "nil",
			getter: StandardStringLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return nil, nil
				},
			},
			want:  nil,
			valid: true,
		},
		{
			name: "invalid type",
			getter: StandardStringLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return make(chan int), nil
				},
			},
			valid:            false,
			expectedErrorMsg: "unsupported type: chan int",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := tt.getter.Get(context.Background(), nil)
			if tt.valid {
				assert.NoError(t, err)
				if tt.want == nil {
					assert.Nil(t, val)
				} else {
					assert.Equal(t, tt.want, *val)
				}
			} else {
				assert.EqualError(t, err, tt.expectedErrorMsg)
			}
		})
	}
}

func Test_StandardFloatGetter(t *testing.T) {
	tests := []struct {
		name             string
		getter           StandardFloatGetter[interface{}]
		want             interface{}
		valid            bool
		expectedErrorMsg string
	}{
		{
			name: "float64 type",
			getter: StandardFloatGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return 1.1, nil
				},
			},
			want:  1.1,
			valid: true,
		},
		{
			name: "ValueTypeFloat type",
			getter: StandardFloatGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return pcommon.NewValueDouble(1.1), nil
				},
			},
			want:  1.1,
			valid: true,
		},
		{
			name: "Incorrect type",
			getter: StandardFloatGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return true, nil
				},
			},
			valid:            false,
			expectedErrorMsg: "expected float64 but got bool",
		},
		{
			name: "nil",
			getter: StandardFloatGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return nil, nil
				},
			},
			valid:            false,
			expectedErrorMsg: "expected float64 but got nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := tt.getter.Get(context.Background(), nil)
			if tt.valid {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, val)
			} else {
				assert.EqualError(t, err, tt.expectedErrorMsg)
			}
		})
	}
}

func Test_StandardFloatLikeGetter(t *testing.T) {
	tests := []struct {
		name             string
		getter           FloatLikeGetter[interface{}]
		want             interface{}
		valid            bool
		expectedErrorMsg string
	}{
		{
			name: "string type",
			getter: StandardFloatLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "1.0", nil
				},
			},
			want:  1.0,
			valid: true,
		},
		{
			name: "int64 type",
			getter: StandardFloatLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return int64(1), nil
				},
			},
			want:  float64(1),
			valid: true,
		},
		{
			name: "float64 type",
			getter: StandardFloatLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return 1.1, nil
				},
			},
			want:  1.1,
			valid: true,
		},
		{
			name: "float64 bool true",
			getter: StandardFloatLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return true, nil
				},
			},
			want:  float64(1),
			valid: true,
		},
		{
			name: "float64 bool false",
			getter: StandardFloatLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return false, nil
				},
			},
			want:  float64(0),
			valid: true,
		},
		{
			name: "pcommon.value type int",
			getter: StandardFloatLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					v := pcommon.NewValueInt(int64(100))
					return v, nil
				},
			},
			want:  float64(100),
			valid: true,
		},
		{
			name: "pcommon.value type float",
			getter: StandardFloatLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					v := pcommon.NewValueDouble(float64(1.1))
					return v, nil
				},
			},
			want:  1.1,
			valid: true,
		},
		{
			name: "pcommon.value type string",
			getter: StandardFloatLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					v := pcommon.NewValueStr("1.1")
					return v, nil
				},
			},
			want:  1.1,
			valid: true,
		},
		{
			name: "pcommon.value type bool true",
			getter: StandardFloatLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					v := pcommon.NewValueBool(true)
					return v, nil
				},
			},
			want:  float64(1),
			valid: true,
		},
		{
			name: "pcommon.value type bool false",
			getter: StandardFloatLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					v := pcommon.NewValueBool(false)
					return v, nil
				},
			},
			want:  float64(0),
			valid: true,
		},
		{
			name: "nil",
			getter: StandardFloatLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return nil, nil
				},
			},
			want:  nil,
			valid: true,
		},
		{
			name: "invalid type",
			getter: StandardFloatLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return []byte{}, nil
				},
			},
			valid:            false,
			expectedErrorMsg: "unsupported type: []uint8",
		},
		{
			name: "invalid pcommon.Value type",
			getter: StandardFloatLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					v := pcommon.NewValueMap()
					return v, nil
				},
			},
			valid:            false,
			expectedErrorMsg: "unsupported value type: Map",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := tt.getter.Get(context.Background(), nil)
			if tt.valid {
				assert.NoError(t, err)
				if tt.want == nil {
					assert.Nil(t, val)
				} else {
					assert.Equal(t, tt.want, *val)
				}
			} else {
				assert.EqualError(t, err, tt.expectedErrorMsg)
			}
		})
	}
}

func Test_StandardIntGetter(t *testing.T) {
	tests := []struct {
		name             string
		getter           StandardIntGetter[interface{}]
		want             interface{}
		valid            bool
		expectedErrorMsg string
	}{
		{
			name: "int64 type",
			getter: StandardIntGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return int64(1), nil
				},
			},
			want:  int64(1),
			valid: true,
		},
		{
			name: "ValueTypeInt type",
			getter: StandardIntGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return pcommon.NewValueInt(1), nil
				},
			},
			want:  int64(1),
			valid: true,
		},
		{
			name: "Incorrect type",
			getter: StandardIntGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return true, nil
				},
			},
			valid:            false,
			expectedErrorMsg: "expected int64 but got bool",
		},
		{
			name: "nil",
			getter: StandardIntGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return nil, nil
				},
			},
			valid:            false,
			expectedErrorMsg: "expected int64 but got nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := tt.getter.Get(context.Background(), nil)
			if tt.valid {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, val)
			} else {
				assert.EqualError(t, err, tt.expectedErrorMsg)
			}
		})
	}
}

func Test_StandardIntLikeGetter(t *testing.T) {
	tests := []struct {
		name             string
		getter           IntLikeGetter[interface{}]
		want             interface{}
		valid            bool
		expectedErrorMsg string
	}{
		{
			name: "string type",
			getter: StandardIntLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "1", nil
				},
			},
			want:  int64(1),
			valid: true,
		},
		{
			name: "int64 type",
			getter: StandardIntLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return int64(1), nil
				},
			},
			want:  int64(1),
			valid: true,
		},
		{
			name: "float64 type",
			getter: StandardIntLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return 1.1, nil
				},
			},
			want:  int64(1),
			valid: true,
		},
		{
			name: "primitive bool true",
			getter: StandardIntLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return true, nil
				},
			},
			want:  int64(1),
			valid: true,
		},
		{
			name: "primitive bool false",
			getter: StandardIntLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return false, nil
				},
			},
			want:  int64(0),
			valid: true,
		},
		{
			name: "pcommon.value type int",
			getter: StandardIntLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					v := pcommon.NewValueInt(int64(100))
					return v, nil
				},
			},
			want:  int64(100),
			valid: true,
		},
		{
			name: "pcommon.value type float",
			getter: StandardIntLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					v := pcommon.NewValueDouble(float64(1.9))
					return v, nil
				},
			},
			want:  int64(1),
			valid: true,
		},
		{
			name: "pcommon.value type string",
			getter: StandardIntLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					v := pcommon.NewValueStr("1")
					return v, nil
				},
			},
			want:  int64(1),
			valid: true,
		},
		{
			name: "pcommon.value type bool true",
			getter: StandardIntLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					v := pcommon.NewValueBool(true)
					return v, nil
				},
			},
			want:  int64(1),
			valid: true,
		},
		{
			name: "pcommon.value type bool false",
			getter: StandardIntLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					v := pcommon.NewValueBool(false)
					return v, nil
				},
			},
			want:  int64(0),
			valid: true,
		},
		{
			name: "nil",
			getter: StandardIntLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return nil, nil
				},
			},
			want:  nil,
			valid: true,
		},
		{
			name: "invalid type",
			getter: StandardIntLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return []byte{}, nil
				},
			},
			valid:            false,
			expectedErrorMsg: "unsupported type: []uint8",
		},
		{
			name: "invalid pcommon.Value type",
			getter: StandardIntLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					v := pcommon.NewValueMap()
					return v, nil
				},
			},
			valid:            false,
			expectedErrorMsg: "unsupported value type: Map",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := tt.getter.Get(context.Background(), nil)
			if tt.valid {
				assert.NoError(t, err)
				if tt.want == nil {
					assert.Nil(t, val)
				} else {
					assert.Equal(t, tt.want, *val)
				}
			} else {
				assert.EqualError(t, err, tt.expectedErrorMsg)
			}
		})
	}
}

func Test_StandardPMapGetter(t *testing.T) {
	tests := []struct {
		name             string
		getter           StandardPMapGetter[interface{}]
		want             interface{}
		valid            bool
		expectedErrorMsg string
	}{
		{
			name: "pcommon.map type",
			getter: StandardPMapGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return pcommon.NewMap(), nil
				},
			},
			want:  pcommon.NewMap(),
			valid: true,
		},
		{
			name: "map[string]any type",
			getter: StandardPMapGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return make(map[string]any), nil
				},
			},
			want:  pcommon.NewMap(),
			valid: true,
		},
		{
			name: "ValueTypeMap type",
			getter: StandardPMapGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return pcommon.NewValueMap(), nil
				},
			},
			want:  pcommon.NewMap(),
			valid: true,
		},
		{
			name: "Incorrect type",
			getter: StandardPMapGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return true, nil
				},
			},
			valid:            false,
			expectedErrorMsg: "expected pcommon.Map but got bool",
		},
		{
			name: "nil",
			getter: StandardPMapGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return nil, nil
				},
			},
			valid:            false,
			expectedErrorMsg: "expected pcommon.Map but got nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := tt.getter.Get(context.Background(), nil)
			if tt.valid {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, val)
			} else {
				assert.EqualError(t, err, tt.expectedErrorMsg)
			}
		})
	}
}
