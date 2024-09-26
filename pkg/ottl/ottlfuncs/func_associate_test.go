package ottlfuncs

import (
	"context"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"testing"
)

func Test_Associate(t *testing.T) {
	type testCase struct {
		name      string
		value     func() any
		keyPath   []string
		valuePath []string
		want      func() pcommon.Map
		wantErr   string
	}
	tests := []testCase{
		{
			name:    "flat object with key path only",
			keyPath: []string{"name"},
			value: func() any {
				sl := pcommon.NewSlice()
				thing1 := sl.AppendEmpty().SetEmptyMap()
				thing1.PutStr("name", "foo")
				thing1.PutInt("value", 2)

				thing2 := sl.AppendEmpty().SetEmptyMap()
				thing2.PutStr("name", "bar")
				thing2.PutInt("value", 5)

				return sl
			},
			want: func() pcommon.Map {
				m := pcommon.NewMap()
				thing1 := m.PutEmptyMap("foo")
				thing1.PutStr("name", "foo")
				thing1.PutInt("value", 2)

				thing2 := m.PutEmptyMap("bar")
				thing2.PutStr("name", "bar")
				thing2.PutInt("value", 5)

				return m
			},
			wantErr: "",
		},
		{
			name:    "flat object with missing key value",
			keyPath: []string{"notfound"},
			value: func() any {
				sl := pcommon.NewSlice()
				thing1 := sl.AppendEmpty().SetEmptyMap()
				thing1.PutStr("name", "foo")
				thing1.PutInt("value", 2)

				thing2 := sl.AppendEmpty().SetEmptyMap()
				thing2.PutStr("name", "bar")
				thing2.PutInt("value", 5)

				return sl
			},
			wantErr: "could not extract key value: provided object does not contain the path [notfound]",
		},
		{
			name:      "flat object with key path and value path",
			keyPath:   []string{"name"},
			valuePath: []string{"value"},
			value: func() any {
				sl := pcommon.NewSlice()
				thing1 := sl.AppendEmpty().SetEmptyMap()
				thing1.PutStr("name", "foo")
				thing1.PutInt("value", 2)

				thing2 := sl.AppendEmpty().SetEmptyMap()
				thing2.PutStr("name", "bar")
				thing2.PutInt("value", 5)

				return sl
			},
			want: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutInt("foo", 2)
				m.PutInt("bar", 5)

				return m
			},
			wantErr: "",
		},
		{
			name:    "nested object with key path only",
			keyPath: []string{"value", "test"},
			value: func() any {
				sl := pcommon.NewSlice()
				thing1 := sl.AppendEmpty().SetEmptyMap()
				thing1.PutStr("name", "foo")
				thing1.PutEmptyMap("value").PutStr("test", "x")

				thing2 := sl.AppendEmpty().SetEmptyMap()
				thing2.PutStr("name", "bar")
				thing2.PutInt("value", 5)
				thing2.PutEmptyMap("value").PutStr("test", "y")

				return sl
			},
			want: func() pcommon.Map {
				m := pcommon.NewMap()
				thing1 := m.PutEmptyMap("x")
				thing1.PutStr("name", "foo")
				thing1.PutEmptyMap("value").PutStr("test", "x")

				thing2 := m.PutEmptyMap("y")
				thing2.PutStr("name", "bar")
				thing2.PutEmptyMap("value").PutStr("test", "y")

				return m
			},
			wantErr: "",
		},
		{
			name:      "nested object with key path and value path",
			keyPath:   []string{"value", "test"},
			valuePath: []string{"name"},
			value: func() any {
				sl := pcommon.NewSlice()
				thing1 := sl.AppendEmpty().SetEmptyMap()
				thing1.PutStr("name", "foo")
				thing1.PutEmptyMap("value").PutStr("test", "x")

				thing2 := sl.AppendEmpty().SetEmptyMap()
				thing2.PutStr("name", "bar")
				thing2.PutInt("value", 5)
				thing2.PutEmptyMap("value").PutStr("test", "y")

				return sl
			},
			want: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("x", "foo")
				m.PutStr("y", "bar")

				return m
			},
			wantErr: "",
		},
		{
			name:    "flat object with key path resolving to non-string",
			keyPath: []string{"value"},
			value: func() any {
				sl := pcommon.NewSlice()
				thing1 := sl.AppendEmpty().SetEmptyMap()
				thing1.PutStr("name", "foo")
				thing1.PutInt("value", 2)

				thing2 := sl.AppendEmpty().SetEmptyMap()
				thing2.PutStr("name", "bar")
				thing2.PutInt("value", 5)

				return sl
			},
			wantErr: "provided key path [value] does not resolve to a string",
		},
		{
			name:      "nested object with value path not resolving to a value",
			keyPath:   []string{"value", "test"},
			valuePath: []string{"notfound"},
			value: func() any {
				sl := pcommon.NewSlice()
				thing1 := sl.AppendEmpty().SetEmptyMap()
				thing1.PutStr("name", "foo")
				thing1.PutEmptyMap("value").PutStr("test", "x")

				thing2 := sl.AppendEmpty().SetEmptyMap()
				thing2.PutStr("name", "bar")
				thing2.PutInt("value", 5)
				thing2.PutEmptyMap("value").PutStr("test", "y")

				return sl
			},
			wantErr: "provided object does not contain the path [notfound]",
		},
		{
			name:      "nested object with value path segment resolving to non-map value",
			keyPath:   []string{"value", "test"},
			valuePath: []string{"name", "nothing"},
			value: func() any {
				sl := pcommon.NewSlice()
				thing1 := sl.AppendEmpty().SetEmptyMap()
				thing1.PutStr("name", "foo")
				thing1.PutEmptyMap("value").PutStr("test", "x")

				thing2 := sl.AppendEmpty().SetEmptyMap()
				thing2.PutStr("name", "bar")
				thing2.PutInt("value", 5)
				thing2.PutEmptyMap("value").PutStr("test", "y")

				return sl
			},
			wantErr: "provided object does not contain the path [name nothing]",
		},
		{
			name:    "unsupported type",
			keyPath: []string{"name"},
			value: func() any {
				return pcommon.NewMap()
			},
			wantErr: "unsupported type provided to SliceToMap function: pcommon.Map",
		},
		{
			name:    "slice containing unsupported value type",
			keyPath: []string{"name"},
			value: func() any {
				sl := pcommon.NewSlice()
				sl.AppendEmpty().SetStr("unsupported")

				return sl
			},
			wantErr: "unsupported value type: string",
		},
		{
			name:    "empty key path",
			keyPath: []string{},
			value: func() any {
				return pcommon.NewMap()
			},
			wantErr: "key path must contain at least one element",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valuePathOptional := ottl.Optional[[]string]{}

			if len(tt.valuePath) > 0 {
				valuePathOptional = ottl.NewTestingOptional(tt.valuePath)
			}
			associateFunc, err := sliceToMapFunction[any](ottl.FunctionContext{}, &SliceToMapArguments[any]{
				Target: &ottl.StandardGetSetter[any]{
					Getter: func(ctx context.Context, tCtx any) (any, error) {
						return tt.value(), nil
					},
				},
				KeyPath:   tt.keyPath,
				ValuePath: valuePathOptional,
			})

			require.NoError(t, err)

			result, err := associateFunc(nil, nil)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			require.EqualValues(t, tt.want().AsRaw(), result.(pcommon.Map).AsRaw())
		})
	}
}
