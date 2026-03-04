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

func Test_ParseSimplifiedXML(t *testing.T) {
	tests := []struct {
		name     string
		document string
		want     pcommon.Map
	}{
		{
			name:     "single leaf",
			document: `<a>b</a>`,
			want: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("a", "b")
				return m
			}(),
		},
		{
			name:     "double leaf",
			document: `<a>b</a><a>c</a>`,
			want: func() pcommon.Map {
				m := pcommon.NewMap()
				b := m.PutEmptySlice("a")
				b.AppendEmpty().SetStr("b")
				b.AppendEmpty().SetStr("c")
				return m
			}(),
		},
		{
			name:     "nested maps",
			document: `<a><b>1</b></a>`,
			want: func() pcommon.Map {
				m := pcommon.NewMap()
				a := m.PutEmptyMap("a")
				a.PutStr("b", "1")
				return m
			}(),
		},
		{
			name:     "mixed slice",
			document: `<a>1</a><a><![CDATA[2]]></a><a><b>3</b></a>`,
			want: func() pcommon.Map {
				m := pcommon.NewMap()
				a := m.PutEmptySlice("a")
				a.AppendEmpty().SetStr("1")
				a.AppendEmpty().SetStr("2")
				b := a.AppendEmpty().SetEmptyMap()
				b.PutStr("b", "3")
				return m
			}(),
		},
		{
			name:     "char data leaf",
			document: `<a><![CDATA[b]]></a>`,
			want: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("a", "b")
				return m
			}(),
		},
		{
			name:     "ignore attributes",
			document: `<a foo="bar"><b hello="world">c</b></a>`,
			want: func() pcommon.Map {
				m := pcommon.NewMap()
				a := m.PutEmptyMap("a")
				a.PutStr("b", "c")
				return m
			}(),
		},
		{
			name:     "ignore declaration",
			document: `<?xml version="1.0" encoding="UTF-8"?><a>b</a>`,
			want: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("a", "b")
				return m
			}(),
		},
		{
			name:     "ignore comments",
			document: `<a><!-- ignore -->b</a><!-- ignore -->`,
			want: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("a", "b")
				return m
			}(),
		},
		{
			name:     "ignore empty other than comment",
			document: `<a><b>2</b><c><!-- c is empty --></c></a>`,
			want: func() pcommon.Map {
				m := pcommon.NewMap()
				a := m.PutEmptyMap("a")
				a.PutStr("b", "2")
				return m
			}(),
		},
		{
			name:     "empty other than comment forces slice",
			document: `<a><b>2</b><c>4</c><c><!-- this c is empty --></c></a>`,
			want: func() pcommon.Map {
				m := pcommon.NewMap()
				a := m.PutEmptyMap("a")
				a.PutStr("b", "2")
				c := a.PutEmptySlice("c")
				c.AppendEmpty().SetStr("4")
				return m
			}(),
		},
		{
			name:     "ignore extraneous text",
			document: `<a>extra1<b>3</b>extra2</a>`,
			want: func() pcommon.Map {
				m := pcommon.NewMap()
				a := m.PutEmptyMap("a")
				a.PutStr("b", "3")
				return m
			}(),
		},
		{
			name:     "ignore extraneous CDATA",
			document: `<a><![CDATA[1]]><b>3</b><![CDATA[2]]></a>`,
			want: func() pcommon.Map {
				m := pcommon.NewMap()
				a := m.PutEmptyMap("a")
				a.PutStr("b", "3")
				return m
			}(),
		},
		{
			name:     "ignore single empty element",
			document: `<a><b>3</b><c/></a>`,
			want: func() pcommon.Map {
				m := pcommon.NewMap()
				a := m.PutEmptyMap("a")
				a.PutStr("b", "3")
				return m
			}(),
		},
		{
			name:     "empty element cascade",
			document: `<a><b><c/></b><d>2</d></a>`,
			want: func() pcommon.Map {
				m := pcommon.NewMap()
				a := m.PutEmptyMap("a")
				a.PutStr("d", "2")
				return m
			}(),
		},
		{
			name:     "empty element forces slice",
			document: `<a><b>3</b><b/></a>`,
			want: func() pcommon.Map {
				m := pcommon.NewMap()
				a := m.PutEmptyMap("a")
				b := a.PutEmptySlice("b")
				b.AppendEmpty().SetStr("3")
				return m
			}(),
		},
		{
			// ParseSimplifiedXML(ConvertAttributesToElementsXML(ConvertTextToElementsXML("<Event>...</Event>")))
			name: "Simplified WEL",
			document: `<Event>
	<xmlns>http://schemas.microsoft.com/win/2004/08/events/event</xmlns>
    <System>
        <Provider><Name>Microsoft-Windows-Security-Auditing</Name><Guid>{54849625-5478-4994-a5ba-3e3b0328c30d}</Guid></Provider>
        <EventID>4625</EventID>
        <Version>0</Version>
        <Level>0</Level>
        <Task>12544</Task>
        <Opcode>0</Opcode>
        <Keywords>0x8010000000000000</Keywords>
        <TimeCreated><SystemTime>2024-09-04T08:38:09.7477579Z</SystemTime></TimeCreated>
        <EventRecordID>1361885</EventRecordID>
        <Correlation><ActivityID>{b67ee0c2-a671-0001-5f6b-82e8c1eeda01}</ActivityID></Correlation>
        <Execution><ProcessID>656</ProcessID><ThreadID>2276</ThreadID></Execution>
        <Channel>Security</Channel>
        <Computer>samuel-vahala</Computer>
        <Security />
    </System>
    <EventData>
		<Data><Name>SubjectUserSid</Name><value>S-1-0-0</value></Data>
		<Data><Name>TargetUserSid</Name><value>S-1-0-0</value></Data>
        <Data><Name>Status</Name><value>0xc000006d</value></Data>
        <Data><Name>WorkstationName</Name><value>D-508</value></Data>
    </EventData>
</Event>`,
			want: func() pcommon.Map {
				result := pcommon.NewMap()
				event := result.PutEmptyMap("Event")
				event.PutStr("xmlns", "http://schemas.microsoft.com/win/2004/08/events/event")
				system := event.PutEmptyMap("System")
				provider := system.PutEmptyMap("Provider")
				provider.PutStr("Name", "Microsoft-Windows-Security-Auditing")
				provider.PutStr("Guid", "{54849625-5478-4994-a5ba-3e3b0328c30d}")
				system.PutStr("EventID", "4625")
				system.PutStr("Version", "0")
				system.PutStr("Level", "0")
				system.PutStr("Task", "12544")
				system.PutStr("Opcode", "0")
				system.PutStr("Keywords", "0x8010000000000000")
				timeCreated := system.PutEmptyMap("TimeCreated")
				timeCreated.PutStr("SystemTime", "2024-09-04T08:38:09.7477579Z")
				system.PutStr("EventRecordID", "1361885")
				correlation := system.PutEmptyMap("Correlation")
				correlation.PutStr("ActivityID", "{b67ee0c2-a671-0001-5f6b-82e8c1eeda01}")
				execution := system.PutEmptyMap("Execution")
				execution.PutStr("ProcessID", "656")
				execution.PutStr("ThreadID", "2276")
				system.PutStr("Channel", "Security")
				system.PutStr("Computer", "samuel-vahala")
				eventData := event.PutEmptyMap("EventData")
				data := eventData.PutEmptySlice("Data")
				data1 := data.AppendEmpty().SetEmptyMap()
				data1.PutStr("Name", "SubjectUserSid")
				data1.PutStr("value", "S-1-0-0")
				data2 := data.AppendEmpty().SetEmptyMap()
				data2.PutStr("Name", "TargetUserSid")
				data2.PutStr("value", "S-1-0-0")
				data3 := data.AppendEmpty().SetEmptyMap()
				data3.PutStr("Name", "Status")
				data3.PutStr("value", "0xc000006d")
				data4 := data.AppendEmpty().SetEmptyMap()
				data4.PutStr("Name", "WorkstationName")
				data4.PutStr("value", "D-508")
				return result
			}(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return tt.document, nil
				},
			}
			exprFunc := parseSimplifiedXML(target)
			result, err := exprFunc(t.Context(), nil)
			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestCreateParseSimplifiedXMLFunc(t *testing.T) {
	factory := NewParseSimplifiedXMLFactory[any]()
	fCtx := ottl.FunctionContext{}

	// Invalid arg type
	exprFunc, err := factory.CreateFunction(fCtx, nil)
	assert.Error(t, err)
	assert.Nil(t, exprFunc)

	// Invalid XML should error on function execution
	exprFunc, err = factory.CreateFunction(
		fCtx, &ParseSimplifiedXMLArguments[any]{
			Target: invalidXMLGetter(),
		})
	require.NoError(t, err)
	assert.NotNil(t, exprFunc)
	_, err = exprFunc(t.Context(), nil)
	assert.Error(t, err)
}
