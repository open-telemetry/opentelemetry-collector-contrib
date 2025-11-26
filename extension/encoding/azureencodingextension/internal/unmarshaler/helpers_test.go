// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package unmarshaler

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestAsTimestamp(t *testing.T) {
	type args struct {
		s       string
		formats []string
	}
	tests := []struct {
		name    string
		args    args
		want    pcommon.Timestamp
		wantErr bool
	}{
		{
			name: "ISO8601 format",
			args: args{
				s: "2025-01-02T12:34:56.000000Z",
			},
			want:    pcommon.NewTimestampFromTime(time.Date(2025, time.January, 2, 12, 34, 56, 0, time.UTC)),
			wantErr: false,
		},
		{
			name: "Custom format",
			args: args{
				s:       "02/03/2025 12:34:56",
				formats: []string{"02/01/2006 15:04:05"},
			},
			want:    pcommon.NewTimestampFromTime(time.Date(2025, time.March, 2, 12, 34, 56, 0, time.UTC)),
			wantErr: false,
		},
		{
			name: "Multiple formats",
			args: args{
				s:       "2025-03-04T12:34:56Z",
				formats: []string{"01/02/2006 15:04:05", "2006-01-02T15:04:05Z"},
			},
			want:    pcommon.NewTimestampFromTime(time.Date(2025, time.March, 4, 12, 34, 56, 0, time.UTC)),
			wantErr: false,
		},
		{
			name: "Fallback format",
			args: args{
				s:       "2025-04-05T12:34:56.000000Z",
				formats: []string{"01/02/2006 15:04:05"},
			},
			want:    pcommon.NewTimestampFromTime(time.Date(2025, time.April, 5, 12, 34, 56, 0, time.UTC)),
			wantErr: false,
		},
		{
			name: "Invalid format",
			args: args{
				s: "string",
			},
			want:    0,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := AsTimestamp(tt.args.s, tt.args.formats...)
			if tt.wantErr {
				require.Error(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAttrPutStrIf(t *testing.T) {
	type args struct {
		key   string
		value string
	}
	tests := []struct {
		name    string
		attrs   pcommon.Map
		args    args
		wantRaw map[string]any
	}{
		{
			name:  "Empty string",
			attrs: pcommon.NewMap(),
			args: args{
				key:   "attr1",
				value: "",
			},
			wantRaw: map[string]any{},
		},
		{
			name:  "Value string",
			attrs: pcommon.NewMap(),
			args: args{
				key:   "attr2",
				value: "value",
			},
			wantRaw: map[string]any{"attr2": "value"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			AttrPutStrIf(tt.attrs, tt.args.key, tt.args.value)
			want := pcommon.NewMap()
			err := want.FromRaw(tt.wantRaw)
			require.NoError(t, err)
			assert.Equal(t, want.AsRaw(), tt.attrs.AsRaw())
		})
	}
}

func TestAttrPutStrPtrIf(t *testing.T) {
	emptyStr := ""
	valueStr := "value"

	type args struct {
		key   string
		value *string
	}
	tests := []struct {
		name    string
		args    args
		wantRaw map[string]any
	}{
		{
			name: "No value",
			args: args{
				key:   "attr1",
				value: nil,
			},
			wantRaw: map[string]any{},
		},
		{
			name: "Empty string",
			args: args{
				key:   "attr2",
				value: &emptyStr,
			},
			wantRaw: map[string]any{},
		},
		{
			name: "Value string",
			args: args{
				key:   "attr3",
				value: &valueStr,
			},
			wantRaw: map[string]any{"attr3": "value"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pcommon.NewMap()
			AttrPutStrPtrIf(got, tt.args.key, tt.args.value)
			want := pcommon.NewMap()
			err := want.FromRaw(tt.wantRaw)
			require.NoError(t, err)
			assert.Equal(t, want.AsRaw(), got.AsRaw())
		})
	}
}

func TestAttrPutIntNumberIf(t *testing.T) {
	t.Parallel()

	type testNumStruct struct {
		Value json.Number `json:"value"`
	}

	tests := []struct {
		name      string
		inputJSON string
		wantRaw   map[string]any
	}{
		{
			name:      "string value",
			inputJSON: `{"value": "3"}`,
			wantRaw: map[string]any{
				"value": int64(3),
			},
		},
		{
			name:      "int value",
			inputJSON: `{"value": 10}`,
			wantRaw: map[string]any{
				"value": int64(10),
			},
		},
		{
			name:      "float value",
			inputJSON: `{"value": 3.14}`,
			wantRaw:   map[string]any{},
		},
		{
			name:      "no value",
			inputJSON: `{"novalue": 15}`,
			wantRaw:   map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ts testNumStruct
			err := json.Unmarshal([]byte(tt.inputJSON), &ts)
			require.NoError(t, err)

			got := pcommon.NewMap()
			AttrPutIntNumberIf(got, "value", ts.Value)
			want := pcommon.NewMap()
			err = want.FromRaw(tt.wantRaw)
			require.NoError(t, err)
			assert.Equal(t, want.AsRaw(), got.AsRaw())
		})
	}
}

func TestAttrPutIntNumberPtrIf(t *testing.T) {
	t.Parallel()

	type testNumStruct struct {
		Value *json.Number `json:"value"`
	}

	tests := []struct {
		name      string
		inputJSON string
		wantRaw   map[string]any
	}{
		{
			name:      "string value",
			inputJSON: `{"value": "3"}`,
			wantRaw: map[string]any{
				"value": int64(3),
			},
		},
		{
			name:      "int value",
			inputJSON: `{"value": 10}`,
			wantRaw: map[string]any{
				"value": int64(10),
			},
		},
		{
			name:      "float value",
			inputJSON: `{"value": 3.14}`,
			wantRaw:   map[string]any{},
		},
		{
			name:      "nil value",
			inputJSON: `{"novalue": 15}`,
			wantRaw:   map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ts testNumStruct
			err := json.Unmarshal([]byte(tt.inputJSON), &ts)
			require.NoError(t, err)

			got := pcommon.NewMap()
			AttrPutIntNumberPtrIf(got, "value", ts.Value)
			want := pcommon.NewMap()
			err = want.FromRaw(tt.wantRaw)
			require.NoError(t, err)
			assert.Equal(t, want.AsRaw(), got.AsRaw())
		})
	}
}

func TestAttrPutMapIf(t *testing.T) {
	t.Parallel()

	type testStruct struct {
		name  string
		value string
	}

	type args struct {
		attrKey   string
		attrValue any
	}
	tests := []struct {
		name    string
		args    args
		wantRaw map[string]any
	}{
		{
			name: "string value",
			args: args{
				attrKey:   "foo",
				attrValue: "bar",
			},
			wantRaw: map[string]any{
				"foo": "bar",
			},
		},
		{
			name: "int value",
			args: args{
				attrKey:   "num",
				attrValue: 42,
			},
			wantRaw: map[string]any{
				"num": 42,
			},
		},
		{
			name: "nil value",
			args: args{
				attrKey:   "nil",
				attrValue: nil,
			},
			wantRaw: map[string]any{},
		},
		{
			name: "slice value",
			args: args{
				attrKey:   "slice",
				attrValue: []any{1, "a"},
			},
			wantRaw: map[string]any{
				"slice": []any{1, "a"},
			},
		},
		{
			name: "map value",
			args: args{
				attrKey:   "map",
				attrValue: map[string]any{"x": 1},
			},
			wantRaw: map[string]any{
				"map": map[string]any{"x": 1},
			},
		},
		{
			name: "struct value",
			args: args{
				attrKey:   "map",
				attrValue: testStruct{name: "test", value: "value"},
			},
			wantRaw: map[string]any{
				"map.original": "{test value}",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pcommon.NewMap()
			AttrPutMapIf(got, tt.args.attrKey, tt.args.attrValue)
			want := pcommon.NewMap()
			err := want.FromRaw(tt.wantRaw)
			require.NoError(t, err)
			assert.Equal(t, want.AsRaw(), got.AsRaw())
		})
	}
}
