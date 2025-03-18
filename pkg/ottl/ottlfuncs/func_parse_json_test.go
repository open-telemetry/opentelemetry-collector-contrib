// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_ParseJSON(t *testing.T) {
	tests := []struct {
		name      string
		target    ottl.StringGetter[any]
		wantMap   func(pcommon.Map)
		wantSlice func(pcommon.Slice)
	}{
		{
			name: "handle string",
			target: ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return `{"test":"string value"}`, nil
				},
			},
			wantMap: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "string value")
			},
		},
		{
			name: "handle bool",
			target: ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return `{"test":true}`, nil
				},
			},
			wantMap: func(expectedMap pcommon.Map) {
				expectedMap.PutBool("test", true)
			},
		},
		{
			name: "handle int",
			target: ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return `{"test":1}`, nil
				},
			},
			wantMap: func(expectedMap pcommon.Map) {
				expectedMap.PutDouble("test", 1)
			},
		},
		{
			name: "handle float",
			target: ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return `{"test":1.1}`, nil
				},
			},
			wantMap: func(expectedMap pcommon.Map) {
				expectedMap.PutDouble("test", 1.1)
			},
		},
		{
			name: "handle nil",
			target: ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return `{"test":null}`, nil
				},
			},
			wantMap: func(expectedMap pcommon.Map) {
				expectedMap.PutEmpty("test")
			},
		},
		{
			name: "handle array",
			target: ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return `{"test":["string","value"]}`, nil
				},
			},
			wantMap: func(expectedMap pcommon.Map) {
				emptySlice := expectedMap.PutEmptySlice("test")
				emptySlice.AppendEmpty().SetStr("string")
				emptySlice.AppendEmpty().SetStr("value")
			},
		},
		{
			name: "handle top level array",
			target: ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return `["string","value"]`, nil
				},
			},
			wantSlice: func(expectedSlice pcommon.Slice) {
				expectedSlice.AppendEmpty().SetStr("string")
				expectedSlice.AppendEmpty().SetStr("value")
			},
		},
		{
			name: "handle top level array of objects",
			target: ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return `[{"test":"value"},{"test":"value"}]`, nil
				},
			},
			wantSlice: func(expectedSlice pcommon.Slice) {
				expectedSlice.AppendEmpty().SetEmptyMap().PutStr("test", "value")
				expectedSlice.AppendEmpty().SetEmptyMap().PutStr("test", "value")
			},
		},
		{
			name: "handle nested object",
			target: ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return `{"test":{"nested":"true"}}`, nil
				},
			},
			wantMap: func(expectedMap pcommon.Map) {
				newMap := expectedMap.PutEmptyMap("test")
				newMap.PutStr("nested", "true")
			},
		},
		{
			name: "updates existing",
			target: ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return `{"existing":"pass"}`, nil
				},
			},
			wantMap: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("existing", "pass")
			},
		},
		{
			name: "complex",
			target: ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return `{"test1":{"nested":"true"},"test2":"string","test3":1,"test4":1.1,"test5":[[1], [2, 3],[]],"test6":null}`, nil
				},
			},
			wantMap: func(expectedMap pcommon.Map) {
				newMap := expectedMap.PutEmptyMap("test1")
				newMap.PutStr("nested", "true")
				expectedMap.PutStr("test2", "string")
				expectedMap.PutDouble("test3", 1)
				expectedMap.PutDouble("test4", 1.1)
				slice := expectedMap.PutEmptySlice("test5")
				slice0 := slice.AppendEmpty().SetEmptySlice()
				slice0.AppendEmpty().SetDouble(1)
				slice1 := slice.AppendEmpty().SetEmptySlice()
				slice1.AppendEmpty().SetDouble(2)
				slice1.AppendEmpty().SetDouble(3)
				slice.AppendEmpty().SetEmptySlice()
				expectedMap.PutEmpty("test6")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := parseJSON(tt.target)
			result, err := exprFunc(context.Background(), nil)
			assert.NoError(t, err)

			if tt.wantMap != nil {
				resultMap, ok := result.(pcommon.Map)
				require.True(t, ok)
				expected := pcommon.NewMap()
				tt.wantMap(expected)
				assert.Equal(t, expected.Len(), resultMap.Len())
				for k := range expected.All() {
					ev, _ := expected.Get(k)
					av, _ := resultMap.Get(k)
					assert.Equal(t, ev, av)
				}
			} else if tt.wantSlice != nil {
				resultSlice, ok := result.(pcommon.Slice)
				require.True(t, ok)
				expected := pcommon.NewSlice()
				tt.wantSlice(expected)
				assert.Equal(t, expected, resultSlice)
			}
		})
	}
}

func Test_ParseJSON_Error(t *testing.T) {
	target := &ottl.StandardStringGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return 1, nil
		},
	}
	exprFunc := parseJSON[any](target)
	_, err := exprFunc(context.Background(), nil)
	assert.Error(t, err)
}

const benchData = `{
	"_id": "667cb0db02f4dfc7648b0f6b",
	"index": 0,
	"guid": "2e419732-8214-4e36-a158-d3ced0217ab6",
	"isActive": true,
	"balance": "$1,105.05",
	"picture": "http://example.com/1",
	"age": 22,
	"eyeColor": "blue",
	"name": "Vincent Knox",
	"gender": "male",
	"company": "ANIVET",
	"email": "vincentknox@anivet.com",
	"phone": "+1 (914) 599-2454",
	"address": "483 Gerritsen Avenue, Succasunna, Massachusetts, 7803",
	"about": "Elit aliqua qui amet duis esse eiusmod cillum proident quis amet elit tempor dolor exercitation. Eu ut tempor exercitation excepteur est. Lorem ad elit sit reprehenderit quis ad sunt laborum amet veniam commodo sit sunt aliqua. Sint incididunt eu ut est magna amet mollit qui deserunt nostrud labore ad. Nostrud officia proident occaecat et irure ut quis culpa mollit veniam. Laboris labore ea reprehenderit veniam mollit enim et proident ipsum id. In qui sit officia laborum.\r\nIn ad consectetur duis ad nisi proident. Non in officia do mollit amet sint voluptate minim nostrud voluptate elit. Veniam Lorem cillum fugiat adipisicing qui ea commodo irure tempor ipsum pariatur sit voluptate. Eiusmod cillum occaecat excepteur cillum aliquip laboris velit aute proident amet.\r\nIpsum sunt eiusmod do ut voluptate sit anim. Consequat nisi nisi consequat amet excepteur ea ad incididunt pariatur veniam exercitation eu ex in. Incididunt sint tempor pariatur Lorem do. Occaecat laborum ad ad id enim dolor deserunt ipsum amet Lorem Lorem. Cillum veniam labore eu do duis.\r\nCillum dolor eiusmod sit amet commodo voluptate pariatur ex irure eu culpa sunt. Incididunt non exercitation est pariatur est. Incididunt mollit Lorem velit ullamco excepteur esse quis id magna et ullamco labore. Laboris consequat tempor est ea amet enim et nisi amet officia dolore magna veniam. Nostrud officia consectetur ea culpa laborum et ut Lorem laboris.\r\nDeserunt labore ullamco dolor exercitation laboris consectetur nulla cupidatat duis. Occaecat quis velit deserunt culpa nostrud eiusmod elit fugiat nulla duis deserunt Lorem do. Proident anim proident aute amet pariatur et do irure. Ad magna qui elit consequat sit exercitation sit. Magna adipisicing id esse aliqua officia magna. Et veniam aliqua minim reprehenderit in culpa. Adipisicing quis eu do Lorem cupidatat consequat ad aute quis.\r\nIn aliquip ea laborum esse dolor reprehenderit qui sit culpa occaecat. Consectetur Lorem dolore adipisicing amet incididunt. Dolor veniam Lorem nulla ex. Eiusmod amet tempor sit eiusmod do reprehenderit proident sit commodo elit cupidatat.\r\nNulla nulla consequat cillum mollit tempor eiusmod irure deserunt amet et voluptate. Fugiat et veniam culpa eiusmod minim ex pariatur. Eiusmod adipisicing pariatur pariatur adipisicing in consequat cillum ut qui veniam amet incididunt ullamco anim.\r\nDolor nulla laborum tempor adipisicing qui id. Exercitation labore aliqua ut laborum velit cupidatat officia. Est qui dolor sint laboris aliqua ea nulla culpa.\r\nAute reprehenderit nulla elit nisi reprehenderit pariatur officia veniam dolore ea occaecat nostrud sunt fugiat. Cillum consequat labore nostrud veniam nisi ea proident est officia incididunt adipisicing qui sint nisi. Ad enim reprehenderit minim labore minim irure dolor. Voluptate commodo dolor excepteur est tempor dolor sunt esse fugiat ea eu et.\r\nIpsum sit velit deserunt aliqua eu labore ad esse eu. Duis eiusmod non exercitation consequat nulla. Enim elit consectetur pariatur sunt labore sunt dolore non do. Sint consequat aliqua tempor consectetur veniam minim. Veniam eu aute occaecat consectetur dolore ullamco dolore officia.\r\n",
	"registered": "2023-06-08T12:29:06 +07:00",
	"latitude": -59.802339,
	"longitude": -160.473187,
	"tags": [
		"pariatur",
		"anim",
		"id",
		"duis",
		"fugiat",
		"qui",
		"veniam"
	],
	"friends": [
		{
			"id": 0,
			"name": "Hester Bruce"
		},
		{
			"id": 1,
			"name": "Laurel Mcknight"
		},
		{
			"id": 2,
			"name": "Wynn Moses"
		}
	],
	"greeting": "Hello, Vincent Knox! You have 1 unread messages.",
	"favoriteFruit": "apple"
}`

func BenchmarkParseJSON(b *testing.B) {
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := parseJSON(ottl.StandardStringGetter[any]{
			Getter: func(_ context.Context, _ any) (any, error) {
				return benchData, nil
			},
		})(ctx, nil)
		require.NoError(b, err)
	}
}
