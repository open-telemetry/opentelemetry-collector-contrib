// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package profiles

import (
	"fmt"
	"iter"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlprofile"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pprofiletest"
)

var (
	TestLogTime      = time.Date(2020, 2, 11, 20, 26, 12, 321, time.UTC)
	TestLogTimestamp = pcommon.NewTimestampFromTime(TestLogTime)

	profileID = [16]byte{2, 1, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
)

func Test_e2e_editors(t *testing.T) {
	tests := []struct {
		statement string
		want      func(t *testing.T, tCtx ottlprofile.TransformContext)
	}{
		{
			statement: `delete_key(attributes, "http.method")`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				removeAttribute(t, tCtx, "http.method")
			},
		},
		{
			statement: `delete_matching_keys(attributes, "^http")`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				removeAttribute(t, tCtx, "http.method")
				removeAttribute(t, tCtx, "http.path")
				removeAttribute(t, tCtx, "http.url")
			},
		},
		{
			statement: `keep_matching_keys(attributes, "^http")`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				removeAttribute(t, tCtx, "flags")
				removeAttribute(t, tCtx, "total.string")
				removeAttribute(t, tCtx, "foo")
				removeAttribute(t, tCtx, "things")
				removeAttribute(t, tCtx, "conflict.conflict1")
				removeAttribute(t, tCtx, "conflict")
			},
		},
		{
			statement: `flatten(attributes)`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				removeAttribute(t, tCtx, "foo")
				removeAttribute(t, tCtx, "conflict.conflict1")
				removeAttribute(t, tCtx, "conflict")

				putProfileAttribute(t, tCtx, "foo.bar", "pass")
				putProfileAttribute(t, tCtx, "foo.flags", "pass")
				putProfileAttribute(t, tCtx, "foo.slice.0", "val")
				putProfileAttribute(t, tCtx, "foo.nested.test", "pass")
				putProfileAttribute(t, tCtx, "conflict.conflict1.conflict2", "nopass")

				removeAttribute(t, tCtx, "things")
				putProfileAttribute(t, tCtx, "things.0.name", "foo")
				putProfileAttribute(t, tCtx, "things.0.value", 2)
				putProfileAttribute(t, tCtx, "things.1.name", "bar")
				putProfileAttribute(t, tCtx, "things.1.value", 5)
			},
		},
		{
			statement: `flatten(attributes, "test")`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				tCtx.GetProfile().AttributeIndices().FromRaw([]int32{})
				putProfileAttribute(t, tCtx, "test.http.method", "get")
				putProfileAttribute(t, tCtx, "test.http.path", "/health")
				putProfileAttribute(t, tCtx, "test.http.url", "http://localhost/health")
				putProfileAttribute(t, tCtx, "test.flags", "A|B|C")
				putProfileAttribute(t, tCtx, "test.total.string", "123456789")
				putProfileAttribute(t, tCtx, "test.foo.bar", "pass")
				putProfileAttribute(t, tCtx, "test.foo.flags", "pass")
				putProfileAttribute(t, tCtx, "test.foo.slice.0", "val")
				putProfileAttribute(t, tCtx, "test.foo.nested.test", "pass")
				putProfileAttribute(t, tCtx, "test.conflict.conflict1.conflict2", "nopass")
				putProfileAttribute(t, tCtx, "test.things.0.name", "foo")
				putProfileAttribute(t, tCtx, "test.things.0.value", 2)
				putProfileAttribute(t, tCtx, "test.things.1.name", "bar")
				putProfileAttribute(t, tCtx, "test.things.1.value", 5)
			},
		},
		{
			statement: `flatten(attributes, "test", resolveConflicts=true)`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				tCtx.GetProfile().AttributeIndices().FromRaw([]int32{})
				putProfileAttribute(t, tCtx, "test.http.method", "get")
				putProfileAttribute(t, tCtx, "test.http.path", "/health")
				putProfileAttribute(t, tCtx, "test.http.url", "http://localhost/health")
				putProfileAttribute(t, tCtx, "test.flags", "A|B|C")
				putProfileAttribute(t, tCtx, "test.total.string", "123456789")
				putProfileAttribute(t, tCtx, "test.foo.bar", "pass")
				putProfileAttribute(t, tCtx, "test.foo.flags", "pass")
				putProfileAttribute(t, tCtx, "test.foo.slice", "val")
				putProfileAttribute(t, tCtx, "test.foo.nested.test", "pass")
				putProfileAttribute(t, tCtx, "test.conflict.conflict1.conflict2", "pass")
				putProfileAttribute(t, tCtx, "test.conflict.conflict1.conflict2.0", "nopass")
				putProfileAttribute(t, tCtx, "test.things.0.name", "foo")
				putProfileAttribute(t, tCtx, "test.things.0.value", 2)
				putProfileAttribute(t, tCtx, "test.things.1.name", "bar")
				putProfileAttribute(t, tCtx, "test.things.1.value", 5)
			},
		},
		{
			statement: `flatten(attributes, depth=1)`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				tCtx.GetProfile().AttributeIndices().FromRaw([]int32{})
				putProfileAttribute(t, tCtx, "http.method", "get")
				putProfileAttribute(t, tCtx, "http.path", "/health")
				putProfileAttribute(t, tCtx, "http.url", "http://localhost/health")
				putProfileAttribute(t, tCtx, "flags", "A|B|C")
				putProfileAttribute(t, tCtx, "total.string", "123456789")
				putProfileAttribute(t, tCtx, "foo.bar", "pass")
				putProfileAttribute(t, tCtx, "foo.flags", "pass")
				putProfileAttribute(t, tCtx, "foo.slice", []any{"val"})
				putProfileAttribute(t, tCtx, "conflict.conflict1.conflict2", "nopass")
				putProfileAttribute(t, tCtx, "conflict.conflict1", map[string]any{"conflict2": "pass"})
				putProfileAttribute(t, tCtx, "things.0", map[string]any{
					"name":  "foo",
					"value": 2,
				})
				putProfileAttribute(t, tCtx, "things.1", map[string]any{
					"name":  "bar",
					"value": 5,
				})
				putProfileAttribute(t, tCtx, "foo.nested", map[string]any{"test": "pass"})
			},
		},
		{
			statement: `keep_keys(attributes, ["flags", "total.string"])`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				removeAttribute(t, tCtx, "http.method")
				removeAttribute(t, tCtx, "http.path")
				removeAttribute(t, tCtx, "http.url")
				removeAttribute(t, tCtx, "foo")
				removeAttribute(t, tCtx, "things")
				removeAttribute(t, tCtx, "conflict.conflict1")
				removeAttribute(t, tCtx, "conflict")
			},
		},
		{
			statement: `limit(attributes, 100, [])`,
			want:      func(_ *testing.T, _ ottlprofile.TransformContext) {},
		},
		{
			statement: `limit(attributes, 1, ["total.string"])`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				removeAttribute(t, tCtx, "http.method")
				removeAttribute(t, tCtx, "http.path")
				removeAttribute(t, tCtx, "http.url")
				removeAttribute(t, tCtx, "flags")
				removeAttribute(t, tCtx, "foo")
				removeAttribute(t, tCtx, "things")
				removeAttribute(t, tCtx, "conflict.conflict1")
				removeAttribute(t, tCtx, "conflict")
			},
		},
		{
			statement: `merge_maps(attributes, attributes["foo"], "insert")`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "bar", "pass")
				putProfileAttribute(t, tCtx, "slice", []any{"val"})
				putProfileAttribute(t, tCtx, "nested", map[string]any{"test": "pass"})
			},
		},
		{
			statement: `merge_maps(attributes, attributes["foo"], "update")`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "flags", "pass")
			},
		},
		{
			statement: `merge_maps(attributes, attributes["foo"], "upsert")`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "bar", "pass")
				putProfileAttribute(t, tCtx, "flags", "pass")
				putProfileAttribute(t, tCtx, "slice", []any{"val"})
				putProfileAttribute(t, tCtx, "nested", map[string]any{"test": "pass"})
			},
		},
		{
			statement: `merge_maps(attributes, {"map_literal": {"list": [{"foo":"bar"}, "test"]}}, "upsert")`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "map_literal", map[string]any{
					"list": []any{
						map[string]any{"foo": "bar"},
						"test",
					},
				})
			},
		},
		{
			statement: `replace_all_matches(attributes, "*/*", "test")`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "http.path", "test")
				putProfileAttribute(t, tCtx, "http.url", "test")
			},
		},
		{
			statement: `replace_all_patterns(attributes, "key", "^http", "test")`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				removeAttribute(t, tCtx, "http.method")
				removeAttribute(t, tCtx, "http.path")
				removeAttribute(t, tCtx, "http.url")
				putProfileAttribute(t, tCtx, "test.method", "get")
				putProfileAttribute(t, tCtx, "test.path", "/health")
				putProfileAttribute(t, tCtx, "test.url", "http://localhost/health")
			},
		},
		{
			statement: `replace_all_patterns(attributes, "value", "/", "@")`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "http.path", "@health")
				putProfileAttribute(t, tCtx, "http.url", "http:@@localhost@health")
			},
		},
		{
			statement: `replace_match(attributes["http.path"], "*/*", "test")`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "http.path", "test")
			},
		},
		{
			statement: `replace_pattern(attributes["http.path"], "/", "@")`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "http.path", "@health")
			},
		},
		{
			statement: `replace_pattern(attributes["http.path"], "/", "@", SHA256)`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "http.path", "c3641f8544d7c02f3580b07c0f9887f0c6a27ff5ab1d4a3e29caf197cfc299aehealth")
			},
		},
		{
			statement: `set(attributes["test"], "pass")`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], nil)`,
			want:      func(_ *testing.T, _ ottlprofile.TransformContext) {},
		},
		{
			statement: `set(attributes["test"], attributes["unknown"])`,
			want:      func(_ *testing.T, _ ottlprofile.TransformContext) {},
		},
		{
			statement: `set(attributes["foo"]["test"], "pass")`,
			want: func(t *testing.T, tCtx ottlprofile.TransformContext) {
				v := pcommon.NewValueEmpty()
				getProfileAttribute(t, tCtx, "foo").CopyTo(v)
				v.Map().PutStr("test", "pass")
				putAttribute(t, tCtx.GetProfilesDictionary(), tCtx.GetProfile(), "foo", v)
			},
		},
		{
			statement: `truncate_all(attributes, 100)`,
			want:      func(_ *testing.T, _ ottlprofile.TransformContext) {},
		},
		{
			statement: `truncate_all(attributes, 1)`,
			want: func(t *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "http.method", "g")
				putProfileAttribute(t, tCtx, "http.path", "/")
				putProfileAttribute(t, tCtx, "http.url", "h")
				putProfileAttribute(t, tCtx, "flags", "A")
				putProfileAttribute(t, tCtx, "total.string", "1")
			},
		},
		{
			statement: `append(attributes["foo"]["slice"], "sample_value")`,
			want: func(t *testing.T, tCtx ottlprofile.TransformContext) {
				v := pcommon.NewValueEmpty()
				getProfileAttribute(t, tCtx, "foo").CopyTo(v)
				mv, _ := v.Map().Get("slice")
				s := mv.Slice()
				s.AppendEmpty().SetStr("sample_value")
				putAttribute(t, tCtx.GetProfilesDictionary(), tCtx.GetProfile(), "foo", v)
			},
		},
		{
			statement: `append(attributes["foo"]["flags"], "sample_value")`,
			want: func(t *testing.T, tCtx ottlprofile.TransformContext) {
				v := pcommon.NewValueEmpty()
				getProfileAttribute(t, tCtx, "foo").CopyTo(v)
				mv, _ := v.Map().Get("flags")
				_ = mv.FromRaw([]any{"pass", "sample_value"})
				putAttribute(t, tCtx.GetProfilesDictionary(), tCtx.GetProfile(), "foo", v)
			},
		},
		{
			statement: `append(attributes["foo"]["slice"], values=[5,6])`,
			want: func(t *testing.T, tCtx ottlprofile.TransformContext) {
				v := pcommon.NewValueEmpty()
				getProfileAttribute(t, tCtx, "foo").CopyTo(v)
				mv, _ := v.Map().Get("slice")
				_ = mv.FromRaw([]any{"val", 5, 6})
				putAttribute(t, tCtx.GetProfilesDictionary(), tCtx.GetProfile(), "foo", v)
			},
		},
		{
			statement: `append(attributes["foo"]["new_slice"], values=[5,6])`,
			want: func(t *testing.T, tCtx ottlprofile.TransformContext) {
				v := pcommon.NewValueEmpty()
				getProfileAttribute(t, tCtx, "foo").CopyTo(v)
				s := v.Map().PutEmptySlice("new_slice")
				_ = s.FromRaw([]any{5, 6})
				putAttribute(t, tCtx.GetProfilesDictionary(), tCtx.GetProfile(), "foo", v)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.statement, func(t *testing.T) {
			statements, err := parseStatementWithAndWithoutPathContext(tt.statement)
			assert.NoError(t, err)

			for _, statement := range statements {
				validator, tCtx := newDictionaryValidator(constructProfileTransformContextEditors())
				_, _, _ = statement.Execute(t.Context(), tCtx)
				require.NoError(t, validator.validate())

				exValidator, exTCtx := newDictionaryValidator(constructProfileTransformContextEditors())
				tt.want(t, exTCtx)
				require.NoError(t, exValidator.validate())

				assert.NoError(t, pprofiletest.CompareResourceProfiles(exTCtx.GetProfilesDictionary(), tCtx.GetProfilesDictionary(), newResourceProfiles(exTCtx), newResourceProfiles(tCtx)))
			}
		})
	}
}

func putAttribute(t *testing.T, dic pprofile.ProfilesDictionary, profile pprofile.Profile, key string, value pcommon.Value) {
	t.Helper()

	kvu := pprofile.NewKeyValueAndUnit()
	keyIdx, err := pprofile.SetString(dic.StringTable(), key)
	require.NoError(t, err)

	kvu.SetKeyStrindex(keyIdx)
	value.CopyTo(kvu.Value())
	idx, err := pprofile.SetAttribute(dic.AttributeTable(), kvu)
	require.NoError(t, err)

	for k, i := range profile.AttributeIndices().All() {
		if i == idx {
			return
		}

		attr := dic.AttributeTable().At(int(i))
		if attr.KeyStrindex() == keyIdx {
			profile.AttributeIndices().SetAt(k, idx)
			return
		}
	}
	profile.AttributeIndices().Append(idx)
}

type table[T any] interface {
	Len() int
	All() iter.Seq2[int, T]
	At(i int) T
}

type dictionaryValidator struct {
	orig, dic pprofile.ProfilesDictionary
}

func newDictionaryValidator(tCtx ottlprofile.TransformContext) (dictionaryValidator, ottlprofile.TransformContext) {
	dv := dictionaryValidator{orig: pprofile.NewProfilesDictionary(), dic: tCtx.GetProfilesDictionary()}
	dv.dic.CopyTo(dv.orig)
	return dv, tCtx
}

// validate ensures that no table entries of a dictionary have been changed. Only new entries are allowed.
func (validator dictionaryValidator) validate() error {
	if err := compareTables(validator.orig.AttributeTable(), validator.dic.AttributeTable()); err != nil {
		return fmt.Errorf("attribute table: %w", err)
	}
	if err := compareTables(validator.orig.LocationTable(), validator.dic.LocationTable()); err != nil {
		return fmt.Errorf("location table: %w", err)
	}
	if err := compareTables(validator.orig.StringTable(), validator.dic.StringTable()); err != nil {
		return fmt.Errorf("string table: %w", err)
	}
	if err := compareTables(validator.orig.AttributeTable(), validator.dic.AttributeTable()); err != nil {
		return fmt.Errorf("attribute table: %w", err)
	}
	if err := compareTables(validator.orig.FunctionTable(), validator.dic.FunctionTable()); err != nil {
		return fmt.Errorf("function table: %w", err)
	}
	if err := compareTables(validator.orig.LinkTable(), validator.dic.LinkTable()); err != nil {
		return fmt.Errorf("link table: %w", err)
	}
	if err := compareTables(validator.orig.MappingTable(), validator.dic.MappingTable()); err != nil {
		return fmt.Errorf("mapping table: %w", err)
	}
	return nil
}

// compareTables compares two tables to ensure that the first is a prefix of the second.
// It means that no changes were made to the original table except new entries.
func compareTables[T any](org, cmp table[T]) error {
	if cmp.Len() < org.Len() {
		return fmt.Errorf("table is smaller than the original: %d < %d", cmp.Len(), org.Len())
	}
	for i, attrOrig := range org.All() {
		attrDic := cmp.At(i)
		if !reflect.DeepEqual(attrOrig, attrDic) {
			return fmt.Errorf("value mismatch at index %d: %v != %v", i, attrOrig, attrDic)
		}
	}
	return nil
}

func Test_e2e_converters(t *testing.T) {
	tests := []struct {
		statement string
		want      func(t *testing.T, tCtx ottlprofile.TransformContext)
		wantErr   bool
		errMsg    string
	}{
		{
			statement: `set(attributes["newOne"], attributes[1])`,
			want:      func(_ *testing.T, _ ottlprofile.TransformContext) {},
			errMsg:    "unable to resolve a string index in map: invalid key type",
		},
		{
			statement: `set(attributes["array"][0.0], "bar")`,
			want:      func(_ *testing.T, _ ottlprofile.TransformContext) {},
			errMsg:    "unable to resolve an integer index in slice: could not resolve key for map/slice, expecting 'int64'",
		},
		{
			statement: `set(attributes["array"][ConvertCase(attributes["A|B|C"], "upper")], "bar")`,
			want:      func(_ *testing.T, _ ottlprofile.TransformContext) {},
			errMsg:    "unable to resolve an integer index in slice: could not resolve key for map/slice, expecting 'int64'",
		},
		{
			statement: `set(attributes[ConvertCase(attributes["A|B|C"], "upper")], "myvalue")`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "SOMETHING", "myvalue")
			},
		},
		{
			statement: `set(attributes[ConvertCase(attributes[attributes["flags"]], "upper")], "myvalue")`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "SOMETHING", "myvalue")
			},
		},
		{
			statement: `set(attributes[attributes["flags"]], "something33")`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "A|B|C", "something33")
			},
		},
		{
			statement: `set(attributes[attributes[attributes["flags"]]], "something2")`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "something", "something2")
			},
		},
		{
			statement: `set(original_payload_format, attributes["things"][Len(attributes["things"]) - 1]["name"])`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				tCtx.GetProfile().SetOriginalPayloadFormat("bar")
			},
		},
		{
			statement: `set(original_payload_format, attributes["things"][attributes["int_value"] + 1]["name"])`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				tCtx.GetProfile().SetOriginalPayloadFormat("bar")
			},
		},
		{
			statement: `set(original_payload_format, attributes[attributes["foo"][attributes["slice"]][attributes["int_value"] + 1 - 1]])`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				tCtx.GetProfile().SetOriginalPayloadFormat("val2")
			},
		},
		{
			statement: `set(original_payload_format, attributes[attributes["foo"][attributes["slice"]][attributes["int_value"]]])`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				tCtx.GetProfile().SetOriginalPayloadFormat("val2")
			},
		},
		{
			statement: `set(resource.attributes[attributes["flags"]], "something33")`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				tCtx.GetResource().Attributes().PutStr("A|B|C", "something33")
			},
		},
		{
			statement: `set(resource.attributes[resource.attributes[attributes["flags"]]], "something33")`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				tCtx.GetResource().Attributes().PutStr("newValue", "something33")
			},
		},
		{
			statement: `set(attributes[resource.attributes[attributes["flags"]]], "something33")`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "newValue", "something33")
			},
		},
		{
			statement: `set(attributes["array"][attributes["int_value"]], 3)`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "array", []any{3})
			},
		},
		{
			statement: `set(attributes["test"], Base64Decode("cGFzcw=="))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], Decode("cGFzcw==", "base64"))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], Concat(["A","B"], ":"))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "A:B")
			},
		},
		{
			statement: `set(attributes["test"], ConvertCase(attributes["http.method"], "upper"))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", http.MethodGet)
			},
		},
		{
			statement: `set(attributes["test"], ConvertCase("PASS", "lower"))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], ConvertCase("fooBar", "snake"))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "foo_bar")
			},
		},
		{
			statement: `set(attributes["test"], ConvertCase("foo_bar", "camel"))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "FooBar")
			},
		},
		{
			statement: `set(attributes["test"], ToCamelCase("foo_bar"))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "FooBar")
			},
		},
		{
			statement: `set(attributes["test"], ToSnakeCase("fooBar"))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "foo_bar")
			},
		},
		{
			statement: `set(attributes["test"], ToUpperCase(attributes["http.method"]))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", http.MethodGet)
			},
		},
		{
			statement: `set(attributes["test"], ToLowerCase("PASS"))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], ConvertAttributesToElementsXML("<Log id=\"1\"><Message>This is a log message!</Message></Log>"))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", `<Log><Message>This is a log message!</Message><id>1</id></Log>`)
			},
		},
		{
			statement: `set(original_payload_format, ConvertTextToElementsXML("<a><b/>foo</a>"))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				tCtx.GetProfile().SetOriginalPayloadFormat("<a><b></b><value>foo</value></a>")
			},
		},
		{
			statement: `set(original_payload_format, ConvertTextToElementsXML("<a><b/>foo</a><c><b/>bar</c>", "/a", "custom"))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				tCtx.GetProfile().SetOriginalPayloadFormat("<a><b></b><custom>foo</custom></a><c><b></b>bar</c>")
			},
		},
		{
			statement: `set(attributes["test"], Double(1.0))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", 1.0)
			},
		},
		{
			statement: `set(attributes["test"], Double("1"))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", 1.0)
			},
		},
		{
			statement: `set(attributes["test"], Double(true))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", 1.0)
			},
		},
		{
			statement: `set(attributes["test"], Double(1))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", 1.0)
			},
		},
		{
			statement: `set(attributes["test"], "pass") where Time("10", "%M") - Time("01", "%M") < Duration("10m")`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], ExtractPatterns("aa123bb", "(?P<numbers>\\d+)"))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", map[string]any{"numbers": "123"})
			},
		},
		{
			statement: `set(attributes["test"], ExtractGrokPatterns("http://user:password@example.com:80/path?query=string", "%{ELB_URI}", true))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", map[string]any{
					"url.scheme":   "http",
					"url.username": "user",
					"url.domain":   "example.com",
					"url.port":     80,
					"url.path":     "/path",
					"url.query":    "query=string",
				})
			},
		},
		{
			statement: `set(attributes["test"], FNV("pass"))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", 266877920130663416)
			},
		},
		{
			statement: `set(attributes["test"], Format("%03d-%s", [7, "test"]))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "007-test")
			},
		},
		{
			statement: `set(attributes["test"], Hour(Time("12", "%H")))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", 12)
			},
		},
		{
			statement: `set(attributes["test"], Hours(Duration("90m")))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", 1.5)
			},
		},
		{
			statement: `set(attributes["test"], InsertXML("<a></a>", "/a", "<b></b>"))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "<a><b></b></a>")
			},
		},
		{
			statement: `set(attributes["test"], Int(1.0))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", 1)
			},
		},
		{
			statement: `set(attributes["test"], Int("1"))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", 1)
			},
		},
		{
			statement: `set(attributes["test"], Int(true))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", 1)
			},
		},
		{
			statement: `set(attributes["test"], Int(1))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", 1)
			},
		},
		{
			statement: `set(attributes["test"], GetXML("<a><b>1</b><c><b>2</b></c></a>", "/a//b"))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "<b>1</b><b>2</b>")
			},
		},
		{
			statement: `set(attributes["test"], Hex(1.0))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "3ff0000000000000")
			},
		},
		{
			statement: `set(attributes["test"], Hex(true))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "01")
			},
		},
		{
			statement: `set(attributes["test"], Hex(12))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "000000000000000c")
			},
		},
		{
			statement: `set(attributes["test"], Hex("12"))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "3132")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where IsBool(false)`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where IsDouble(1.0)`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where IsMap(attributes["foo"])`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where IsList(attributes["foo"]["slice"])`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where IsMatch("aa123bb", "\\d{3}")`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where IsString("")`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], Len(attributes["foo"]))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", 4)
			},
		},
		{
			statement: `set(attributes["test"], Log(1))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", 0.0)
			},
		},
		{
			statement: `set(attributes["test"], IsValidLuhn("17893729974"))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", true)
			},
		},
		{
			statement: `set(attributes["test"], IsValidLuhn(17893729975))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", false)
			},
		},
		{
			statement: `set(attributes["test"], MD5("pass"))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "1a1dc91c907325c69271ddf0c944bc72")
			},
		},
		{
			statement: `set(attributes["test"], Microseconds(Duration("1ms")))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", 1000)
			},
		},
		{
			statement: `set(attributes["test"], Milliseconds(Duration("1s")))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", 1000)
			},
		},
		{
			statement: `set(attributes["test"], Minutes(Duration("1h")))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", 60.0)
			},
		},
		{
			statement: `set(attributes["test"], Murmur3Hash128("Hello World"))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "dbc2a0c1ab26631a27b4c09fcf1fe683")
			},
		},
		{
			statement: `set(attributes["test"], Murmur3Hash("Hello World"))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "ce837619")
			},
		},
		{
			statement: `set(attributes["test"], Nanoseconds(Duration("1ms")))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", 1000000)
			},
		},
		{
			statement: `set(attributes["test"], "pass") where Now() - Now() < Duration("1h")`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], ParseCSV("val1;val2;val3","header1|header2|header3",";","|","strict"))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", map[string]any{
					"header1": "val1",
					"header2": "val2",
					"header3": "val3",
				})
			},
		},
		{
			statement: `set(attributes["test"], ParseCSV("val1,val2,val3","header1|header2|header3",headerDelimiter="|",mode="strict"))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", map[string]any{
					"header1": "val1",
					"header2": "val2",
					"header3": "val3",
				})
			},
		},
		{
			statement: `set(attributes["test"], ParseJSON("{\"id\":1}"))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", map[string]any{"id": 1.0})
			},
		},
		{
			statement: `set(attributes["test"], ParseJSON("[\"value1\",\"value2\"]"))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", []any{"value1", "value2"})
			},
		},
		{
			statement: `set(attributes["test"], ParseKeyValue("k1=v1 k2=v2"))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", map[string]any{
					"k1": "v1",
					"k2": "v2",
				})
			},
		},
		{
			statement: `set(attributes["test"], ParseKeyValue("k1!v1_k2!v2", "!", "_"))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", map[string]any{
					"k1": "v1",
					"k2": "v2",
				})
			},
		},
		{
			statement: `set(attributes["test"], ParseKeyValue("k1!v1_k2!\"v2__!__v2\"", "!", "_"))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", map[string]any{
					"k1": "v1",
					"k2": "v2__!__v2",
				})
			},
		},
		{
			statement: `set(attributes["test"], ToKeyValueString(ParseKeyValue("k1=v1 k2=v2"), "=", " ", true))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "k1=v1 k2=v2")
			},
		},
		{
			statement: `set(attributes["test"], ToKeyValueString(ParseKeyValue("k1:v1,k2:v2", ":" , ","), ":", ",", true))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "k1:v1,k2:v2")
			},
		},
		{
			statement: `set(attributes["test"], ToKeyValueString(ParseKeyValue("k1=v1 k2=v2"), "!", "+", true))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "k1!v1+k2!v2")
			},
		},
		{
			statement: `set(attributes["test"], ToKeyValueString(ParseKeyValue("k1=v1 k2=v2=v3"), "=", " ", true))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "k1=v1 k2=\"v2=v3\"")
			},
		},
		{
			statement: `set(attributes["test"], ParseSimplifiedXML("<Log><id>1</id><Message>This is a log message!</Message></Log>"))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", map[string]any{
					"Log": map[string]any{
						"id":      "1",
						"Message": "This is a log message!",
					},
				})
			},
		},
		{
			statement: `set(attributes["test"], ParseXML("<Log id=\"1\"><Message>This is a log message!</Message></Log>"))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", map[string]any{
					"tag": "Log",
					"attributes": map[string]any{
						"id": "1",
					},
					"children": []any{
						map[string]any{
							"tag":     "Message",
							"content": "This is a log message!",
						},
					},
				})
			},
		},
		{
			statement: `set(attributes["test"], RemoveXML("<Log id=\"1\"><Message>This is a log message!</Message></Log>", "/Log/Message"))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", `<Log id="1"></Log>`)
			},
		},
		{
			statement: `set(attributes["test"], Seconds(Duration("1m")))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", 60.0)
			},
		},
		{
			statement: `set(attributes["test"], SHA1("pass"))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "9d4e1e23bd5b727046a9e3b4b7db57bd8d6ee684")
			},
		},
		{
			statement: `set(attributes["test"], SHA256("pass"))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "d74ff0ee8da3b9806b18c877dbf29bbde50b5bd8e4dad7a3a725000feb82e8f1")
			},
		},
		{
			statement: `set(attributes["test"], SHA512("pass"))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "5b722b307fce6c944905d132691d5e4a2214b7fe92b738920eb3fce3a90420a19511c3010a0e7712b054daef5b57bad59ecbd93b3280f210578f547f4aed4d25")
			},
		},
		{
			statement: `set(attributes["test"], Sort(Split(attributes["flags"], "|"), "desc"))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", []any{"C", "B", "A"})
			},
		},
		{
			statement: `set(attributes["test"], Sort([true, false, false]))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", []any{false, false, true})
			},
		},
		{
			statement: `set(attributes["test"], Sort([3, 6, 9], "desc"))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", []any{9, 6, 3})
			},
		},
		{
			statement: `set(attributes["test"], Sort([Double(1.5), Double(10.2), Double(2.3), Double(0.5)]))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", []any{0.5, 1.5, 2.3, 10.2})
			},
		},
		{
			statement: `set(attributes["test"], Sort([Int(11), Double(2.2), Double(-1)]))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", []any{-1.0, 2.2, 11})
			},
		},
		{
			statement: `set(attributes["test"], Sort([false, Int(11), Double(2.2), "three"]))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", []any{11, 2.2, false, "three"})
			},
		},
		{
			statement: `set(profile_id, ProfileID(0x01000000000000000000000000000000))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				tCtx.GetProfile().SetProfileID(pprofile.ProfileID{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
			},
		},
		{
			statement: `set(attributes["test"], Split(attributes["flags"], "|"))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", []any{"A", "B", "C"})
			},
		},
		{
			statement: `set(attributes["test"], String("test"))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "test")
			},
		},
		{
			statement: `set(attributes["test"], String(attributes["http.method"]))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "get")
			},
		},
		{
			statement: `set(attributes["test"], String(profile_id))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "[2,1,3,4,5,6,7,8,9,10,11,12,13,14,15,16]")
			},
		},
		{
			statement: `set(attributes["test"], String([1,2,3]))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "[1,2,3]")
			},
		},
		{
			statement: `set(attributes["test"], String(true))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "true")
			},
		},
		{
			statement: `set(attributes["test"], Substring("pass", 0, 2))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "pa")
			},
		},
		{
			statement: `set(time, TruncateTime(time, Duration("1s")))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				tCtx.GetProfile().SetTime(pcommon.NewTimestampFromTime(TestLogTimestamp.AsTime().Truncate(time.Second)))
			},
		},
		{
			statement: `set(attributes["time"], FormatTime(time, "%Y-%m-%d"))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "time", "2020-02-11")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where UnixMicro(time) > 0`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where UnixMilli(time) > 0`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where UnixNano(time) > 0`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where UnixSeconds(time) > 0`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], "pass") where IsString(UUID())`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "pass")
			},
		},
		{
			statement: `set(attributes["test"], "\\")`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "\\")
			},
		},
		{
			statement: `set(attributes["test"], "\\\\")`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "\\\\")
			},
		},
		{
			statement: `set(attributes["test"], "\\\\\\")`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "\\\\\\")
			},
		},
		{
			statement: `set(attributes["test"], "\\\\\\\\")`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "\\\\\\\\")
			},
		},
		{
			statement: `set(attributes["test"], "\"")`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", `"`)
			},
		},
		{
			statement: `keep_keys(attributes["foo"], ["\\", "bar"])`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				// keep_keys should see two arguments
				putProfileAttribute(t, tCtx, "foo", map[string]any{"bar": "pass"})
			},
		},
		{
			statement: `set(attributes["test"], UserAgent("curl/7.81.0"))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", map[string]any{
					"user_agent.original": "curl/7.81.0",
					"user_agent.name":     "curl",
					"user_agent.version":  "7.81.0",
					"os.name":             "Other",
				})
			},
		},
		{
			statement: `set(attributes["test"], SliceToMap(attributes["things"], ["name"]))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", map[string]any{
					"foo": map[string]any{"name": "foo", "value": 2},
					"bar": map[string]any{"name": "bar", "value": 5},
				})
			},
		},
		{
			statement: `set(attributes["test"], SliceToMap(attributes["things"], ["name"], ["value"]))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", map[string]any{
					"foo": 2,
					"bar": 5,
				})
			},
		},
		{
			statement: `set(attributes["test"], {"list":[{"foo":"bar"}]})`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", map[string]any{
					"list": []any{
						map[string]any{"foo": "bar"},
					},
				})
			},
		},
		{
			statement: `set(attributes, {"list":[{"foo":"bar"}]})`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				tCtx.GetProfile().AttributeIndices().FromRaw([]int32{})
				putProfileAttribute(t, tCtx, "list", []any{
					map[string]any{"foo": "bar"},
				})
			},
		},
		{
			statement: `set(attributes["arr"], [{"list":[{"foo":"bar"}]}, {"bar":"baz"}])`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "arr", []any{
					map[string]any{"list": []any{map[string]any{"foo": "bar"}}},
					map[string]any{"bar": "baz"},
				})
			},
		},
		{
			statement: `set(attributes["test"], IsList([{"list":[{"foo":"bar"}]}, {"bar":"baz"}]))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", true)
			},
		},
		{
			statement: `set(attributes["test"], IsMap({"list":[{"foo":"bar"}]}))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", true)
			},
		},
		{
			statement: `set(attributes["test"], Len([{"list":[{"foo":"bar"}]}, {"bar":"baz"}]))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", 2)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.statement, func(t *testing.T) {
			statements, err := parseStatementWithAndWithoutPathContext(tt.statement)
			assert.NoError(t, err)

			for _, statement := range statements {
				tCtx := constructProfileTransformContext()
				_, _, err = statement.Execute(t.Context(), tCtx)
				if tt.errMsg == "" {
					assert.NoError(t, err)
				} else if err != nil {
					assert.Contains(t, err.Error(), tt.errMsg)
				}

				exTCtx := constructProfileTransformContext()
				tt.want(t, exTCtx)

				assert.NoError(t, pprofiletest.CompareResourceProfiles(exTCtx.GetProfilesDictionary(), tCtx.GetProfilesDictionary(), newResourceProfiles(exTCtx), newResourceProfiles(tCtx)))
			}
		})
	}
}

func Test_e2e_ottl_features(t *testing.T) {
	tests := []struct {
		name      string
		statement string
		want      func(t *testing.T, tCtx ottlprofile.TransformContext)
	}{
		{
			name:      "where clause",
			statement: `set(attributes["test"], "pass") where original_payload_format == "operationB"`,
			want:      func(_ *testing.T, _ ottlprofile.TransformContext) {},
		},
		{
			name:      "reach upwards",
			statement: `set(attributes["test"], "pass") where resource.attributes["host.name"] == "localhost"`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "pass")
			},
		},
		{
			name:      "where clause with dynamic indexing",
			statement: `set(attributes["foo"], "bar") where attributes[attributes["flags"]] != nil`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "foo", "bar")
			},
		},
		{
			name:      "Using hex",
			statement: `set(attributes["test"], "pass") where profile_id == ProfileID(0x0201030405060708090a0b0c0d0e0f10)`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "pass")
			},
		},
		{
			name:      "where clause without comparator",
			statement: `set(attributes["test"], "pass") where IsMatch(original_payload_format, "operation[AC]")`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "pass")
			},
		},
		{
			name:      "where clause with Converter return value",
			statement: `set(attributes["test"], "pass") where original_payload_format == Concat(["operation", "A"], "")`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "pass")
			},
		},
		{
			name:      "composing functions",
			statement: `merge_maps(attributes, ParseJSON("{\"json_test\":\"pass\"}"), "insert") where original_payload_format == "operationA"`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "json_test", "pass")
			},
		},
		{
			name:      "complex indexing found",
			statement: `set(attributes["test"], attributes["foo"]["bar"])`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", "pass")
			},
		},
		{
			name:      "complex indexing not found",
			statement: `set(attributes["test"], attributes["metadata"]["uid"])`,
			want:      func(_ *testing.T, _ ottlprofile.TransformContext) {},
		},
		{
			name:      "map value as input to function",
			statement: `set(attributes["isMap"], IsMap({"foo": {"bar": "baz", "test": "pass"}}))`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "isMap", true)
			},
		},
		{
			name:      "extract value from Split function result slice of type []string",
			statement: `set(attributes["my.environment.2"], Split(resource.attributes["host.name"],"h")[1])`,
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "my.environment.2", "ost")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statements, err := parseStatementWithAndWithoutPathContext(tt.statement)
			assert.NoError(t, err)

			for _, statement := range statements {
				tCtx := constructProfileTransformContext()
				_, _, _ = statement.Execute(t.Context(), tCtx)

				exTCtx := constructProfileTransformContext()
				tt.want(t, exTCtx)

				assert.NoError(t, pprofiletest.CompareResourceProfiles(exTCtx.GetProfilesDictionary(), tCtx.GetProfilesDictionary(), newResourceProfiles(exTCtx), newResourceProfiles(tCtx)))
			}
		})
	}
}

func Test_e2e_ottl_statement_sequence(t *testing.T) {
	tests := []struct {
		name       string
		statements []string
		want       func(t *testing.T, tCtx ottlprofile.TransformContext)
	}{
		{
			name: "delete key of map literal",
			statements: []string{
				`set(attributes["test"], {"foo":"bar", "list":[{"test":"hello"}]})`,
				`delete_key(attributes["test"], "foo")`,
			},
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", map[string]any{"list": []any{map[string]any{"test": "hello"}}})
			},
		},
		{
			name: "delete matching keys of map literal",
			statements: []string{
				`set(attributes["test"], {"foo":"bar", "list":[{"test":"hello"}]})`,
				`delete_matching_keys(attributes["test"], ".*oo")`,
			},
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", map[string]any{"list": []any{map[string]any{"test": "hello"}}})
			},
		},
		{
			name: "keep matching keys of map literal",
			statements: []string{
				`set(attributes["test"], {"foo":"bar", "list":[{"test":"hello"}]})`,
				`keep_matching_keys(attributes["test"], ".*ist")`,
			},
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", map[string]any{"list": []any{map[string]any{"test": "hello"}}})
			},
		},
		{
			name: "flatten map literal",
			statements: []string{
				`set(attributes["test"], {"foo":"bar", "list":[{"test":"hello"}]})`,
				`flatten(attributes["test"])`,
			},
			want: func(_ *testing.T, tCtx ottlprofile.TransformContext) {
				putProfileAttribute(t, tCtx, "test", map[string]any{"foo": "bar", "list.0.test": "hello"})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tCtx := constructProfileTransformContext()

			for _, statement := range tt.statements {
				statements, err := parseStatementWithAndWithoutPathContext(statement)
				assert.NoError(t, err)

				for _, s := range statements {
					_, _, _ = s.Execute(t.Context(), tCtx)
				}
			}

			exTCtx := constructProfileTransformContext()
			tt.want(t, exTCtx)

			assert.NoError(t, pprofiletest.CompareResourceProfiles(exTCtx.GetProfilesDictionary(), tCtx.GetProfilesDictionary(), newResourceProfiles(exTCtx), newResourceProfiles(tCtx)))
		})
	}
}

func Test_e2e_ottl_value_expressions(t *testing.T) {
	tests := []struct {
		name      string
		statement string
		want      func() any
	}{
		{
			name:      "string literal",
			statement: `"foo"`,
			want: func() any {
				return "foo"
			},
		},
		{
			name:      "attribute value",
			statement: `resource.attributes["host.name"]`,
			want: func() any {
				return "localhost"
			},
		},
		{
			name:      "Using converter",
			statement: `ProfileID(0x0102030405060708090a0b0c0d0e0f10)`,
			want: func() any {
				return pprofile.ProfileID{0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8, 0x9, 0xa, 0xb, 0xc, 0xd, 0xe, 0xf, 0x10}
			},
		},
		{
			name:      "Adding results of two converter operations",
			statement: `Len(attributes) + Len(attributes)`,
			want: func() any {
				return int64(28)
			},
		},
		{
			name:      "Nested converter operations",
			statement: `Hex(Len(attributes) + Len(attributes))`,
			want: func() any {
				return "000000000000001c"
			},
		},
		{
			name:      "return map type 1",
			statement: `attributes["foo"]`,
			want: func() any {
				m := pcommon.NewMap()
				_ = m.FromRaw(map[string]any{
					"bar": "pass",
				})
				return m
			},
		},
		{
			name:      "return map type 2",
			statement: `attributes["foo2"]`,
			want: func() any {
				m := pcommon.NewMap()
				_ = m.FromRaw(map[string]any{
					"slice": []any{
						"val",
					},
				})
				return m
			},
		},
		{
			name:      "return map type 3",
			statement: `attributes["foo3"]`,
			want: func() any {
				m := pcommon.NewMap()
				_ = m.FromRaw(map[string]any{
					"nested": map[string]any{
						"test": "pass",
					},
				})
				return m
			},
		},
		{
			name:      "return list",
			statement: `attributes["things"]`,
			want: func() any {
				s := pcommon.NewSlice()
				_ = s.FromRaw([]any{
					map[string]any{"name": "foo"},
					map[string]any{"name": "bar"},
				})
				return s
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := componenttest.NewNopTelemetrySettings()

			profileParser, err := ottlprofile.NewParser(ottlfuncs.StandardFuncs[ottlprofile.TransformContext](), settings)
			assert.NoError(t, err)
			valueExpr, err := profileParser.ParseValueExpression(tt.statement)
			assert.NoError(t, err)

			tCtx := constructProfileTransformContextValueExpressions()
			val, err := valueExpr.Eval(t.Context(), tCtx)
			assert.NoError(t, err)

			assert.Equal(t, tt.want(), val)
		})
	}
}

func parseStatementWithAndWithoutPathContext(statement string) ([]*ottl.Statement[ottlprofile.TransformContext], error) {
	settings := componenttest.NewNopTelemetrySettings()
	parserWithoutPathCtx, err := ottlprofile.NewParser(ottlfuncs.StandardFuncs[ottlprofile.TransformContext](), settings)
	if err != nil {
		return nil, err
	}

	withoutPathCtxResult, err := parserWithoutPathCtx.ParseStatement(statement)
	if err != nil {
		return nil, err
	}

	parserWithPathCtx, err := ottlprofile.NewParser(ottlfuncs.StandardFuncs[ottlprofile.TransformContext](), settings, ottlprofile.EnablePathContextNames())
	if err != nil {
		return nil, err
	}

	pc, err := ottl.NewParserCollection(settings,
		ottl.WithParserCollectionContext[ottlprofile.TransformContext, *ottl.Statement[ottlprofile.TransformContext]](
			ottlprofile.ContextName,
			&parserWithPathCtx,
			ottl.WithStatementConverter(func(_ *ottl.ParserCollection[*ottl.Statement[ottlprofile.TransformContext]], _ ottl.StatementsGetter, parsedStatements []*ottl.Statement[ottlprofile.TransformContext]) (*ottl.Statement[ottlprofile.TransformContext], error) {
				return parsedStatements[0], nil
			})))
	if err != nil {
		return nil, err
	}

	withPathCtxResult, err := pc.ParseStatementsWithContext(ottlprofile.ContextName, ottl.NewStatementsGetter([]string{statement}), true)
	if err != nil {
		return nil, err
	}

	return []*ottl.Statement[ottlprofile.TransformContext]{withoutPathCtxResult, withPathCtxResult}, nil
}

func constructProfileTransformContext() ottlprofile.TransformContext {
	resource := pcommon.NewResource()
	resource.Attributes().PutStr("host.name", "localhost")
	resource.Attributes().PutStr("A|B|C", "newValue")

	scope := pcommon.NewInstrumentationScope()
	scope.SetName("scope")

	profile := &pprofiletest.Profile{
		TimeNanos:              TestLogTimestamp,
		DurationNanos:          1000,
		DroppedAttributesCount: 1,
		OriginalPayloadFormat:  "operationA",
		ProfileID:              profileID,
		Attributes: []pprofiletest.Attribute{
			{Key: "http.method", Value: "get"},
			{Key: "http.path", Value: "/health"},
			{Key: "http.url", Value: "http://localhost/health"},
			{Key: "flags", Value: "A|B|C"},
			{Key: "total.string", Value: "123456789"},
			{Key: "A|B|C", Value: "something"},
			{Key: "foo", Value: "foo"},
			{Key: "slice", Value: "slice"},
			{Key: "val", Value: "val2"},
			{Key: "int_value", Value: 0},
			{Key: "array", Value: []any{"looong"}},
			{Key: "foo", Value: map[string]any{"bar": "pass", "flags": "pass", "slice": []any{"val"}, "nested": map[string]any{"test": "pass"}}},
			{Key: "things", Value: []any{
				map[string]any{"name": "foo", "value": 2},
				map[string]any{"name": "bar", "value": 5},
			}},
		},
	}

	dic := pprofile.NewProfilesDictionary()
	scopeProfiles := pprofile.NewScopeProfiles()
	return ottlprofile.NewTransformContext(profile.Transform(dic, scopeProfiles), dic, scope, resource, scopeProfiles, pprofile.NewResourceProfiles())
}

func constructProfileTransformContextEditors() ottlprofile.TransformContext {
	resource := pcommon.NewResource()
	resource.Attributes().PutStr("host.name", "localhost")

	scope := pcommon.NewInstrumentationScope()
	scope.SetName("scope")

	profile := &pprofiletest.Profile{
		TimeNanos:              TestLogTimestamp,
		DurationNanos:          1000,
		DroppedAttributesCount: 1,
		OriginalPayloadFormat:  "operationA",
		ProfileID:              profileID,
		Attributes: []pprofiletest.Attribute{
			{Key: "http.method", Value: "get"},
			{Key: "http.path", Value: "/health"},
			{Key: "http.url", Value: "http://localhost/health"},
			{Key: "flags", Value: "A|B|C"},
			{Key: "total.string", Value: "123456789"},
			{Key: "conflict", Value: map[string]any{"conflict1": map[string]any{"conflict2": "pass"}}},
			{Key: "conflict.conflict1", Value: map[string]any{"conflict2": "nopass"}},
			{Key: "foo", Value: map[string]any{"bar": "pass", "flags": "pass", "slice": []any{"val"}, "nested": map[string]any{"test": "pass"}}},
			{Key: "things", Value: []any{
				map[string]any{"name": "foo", "value": 2},
				map[string]any{"name": "bar", "value": 5},
			}},
		},
	}

	dic := pprofile.NewProfilesDictionary()
	scopeProfiles := pprofile.NewScopeProfiles()
	return ottlprofile.NewTransformContext(profile.Transform(dic, scopeProfiles), dic, scope, resource, scopeProfiles, pprofile.NewResourceProfiles())
}

func constructProfileTransformContextValueExpressions() ottlprofile.TransformContext {
	resource := pcommon.NewResource()
	resource.Attributes().PutStr("host.name", "localhost")
	resource.Attributes().PutStr("A|B|C", "newValue")

	scope := pcommon.NewInstrumentationScope()
	scope.SetName("scope")

	profile := &pprofiletest.Profile{
		TimeNanos:              TestLogTimestamp,
		DurationNanos:          1000,
		DroppedAttributesCount: 1,
		Attributes: []pprofiletest.Attribute{
			{Key: "http.method", Value: "get"},
			{Key: "http.path", Value: "/health"},
			{Key: "http.url", Value: "http://localhost/health"},
			{Key: "flags", Value: "A|B|C"},
			{Key: "total.string", Value: "123456789"},
			{Key: "A|B|C", Value: "something"},
			{Key: "foo", Value: "foo"},
			{Key: "slice", Value: "slice"},
			{Key: "val", Value: "val2"},
			{Key: "int_value", Value: 0},
			{Key: "array", Value: []any{"looong"}},
			{Key: "foo", Value: map[string]any{"bar": "pass"}},
			{Key: "foo2", Value: map[string]any{"slice": []any{"val"}}},
			{Key: "foo3", Value: map[string]any{"nested": map[string]any{"test": "pass"}}},
			{Key: "things", Value: []any{
				map[string]any{"name": "foo"},
				map[string]any{"name": "bar"},
			}},
		},
	}

	dic := pprofile.NewProfilesDictionary()
	scopeProfiles := pprofile.NewScopeProfiles()
	return ottlprofile.NewTransformContext(profile.Transform(dic, scopeProfiles), dic, scope, resource, scopeProfiles, pprofile.NewResourceProfiles())
}

func newResourceProfiles(tCtx ottlprofile.TransformContext) pprofile.ResourceProfiles {
	rp := pprofile.NewResourceProfiles()
	tCtx.GetResource().CopyTo(rp.Resource())
	sp := rp.ScopeProfiles().AppendEmpty()
	tCtx.GetInstrumentationScope().CopyTo(sp.Scope())
	p := sp.Profiles().AppendEmpty()
	tCtx.GetProfile().CopyTo(p)
	return rp
}

func putProfileAttribute(t *testing.T, tCtx ottlprofile.TransformContext, key string, value any) {
	dic := tCtx.GetProfilesDictionary()
	profile := tCtx.GetProfile()
	switch v := value.(type) {
	case string:
		putAttribute(t, tCtx.GetProfilesDictionary(), tCtx.GetProfile(), key, pcommon.NewValueStr(v))
	case float64:
		putAttribute(t, tCtx.GetProfilesDictionary(), tCtx.GetProfile(), key, pcommon.NewValueDouble(v))
	case int:
		putAttribute(t, tCtx.GetProfilesDictionary(), tCtx.GetProfile(), key, pcommon.NewValueInt(int64(v)))
	case bool:
		putAttribute(t, tCtx.GetProfilesDictionary(), tCtx.GetProfile(), key, pcommon.NewValueBool(v))
	case []any:
		sl := pcommon.NewValueSlice()
		require.NoError(t, sl.FromRaw(v))
		putAttribute(t, dic, profile, key, sl)
	case map[string]any:
		m := pcommon.NewValueMap()
		require.NoError(t, m.FromRaw(v))
		putAttribute(t, dic, profile, key, m)
	default:
		t.Fatalf("unsupported value type: %T", v)
	}
}

func removeAttribute(t *testing.T, tCtx ottlprofile.TransformContext, key string) {
	table := tCtx.GetProfilesDictionary().AttributeTable()
	indices := tCtx.GetProfile().AttributeIndices().AsRaw()

	idx := findAttributeIndex(tCtx.GetProfilesDictionary(), table, indices, key)
	if idx == -1 {
		t.Fatalf("attribute %s not found", key)
		return
	}

	if idx < len(indices)-1 {
		copy(indices[idx:], indices[idx+1:])
	}
	tCtx.GetProfile().AttributeIndices().FromRaw(indices[:len(indices)-1])
}

//nolint:unparam // Silence linter to keep key as param for future changes.
func getProfileAttribute(t *testing.T, tCtx ottlprofile.TransformContext, key string) pcommon.Value {
	table := tCtx.GetProfilesDictionary().AttributeTable()
	indices := tCtx.GetProfile().AttributeIndices().AsRaw()

	idx := findAttributeIndex(tCtx.GetProfilesDictionary(), table, indices, key)
	if idx == -1 {
		t.Fatalf("attribute %s not found", key)
	}

	return table.At(int(indices[idx])).Value()
}

func findAttributeIndex(dic pprofile.ProfilesDictionary, table pprofile.KeyValueAndUnitSlice, indices []int32, key string) int {
	for i, tableIndex := range indices {
		attr := table.At(int(tableIndex))
		if dic.StringTable().At(int(attr.KeyStrindex())) == key {
			return i
		}
	}
	return -1
}
