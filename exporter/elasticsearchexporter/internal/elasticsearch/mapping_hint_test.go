// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearch

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestHasHint(t *testing.T) {
	tests := []struct {
		name      string
		attrsFunc func() pcommon.Map
		hint      MappingHint
		want      bool
	}{
		{
			name:      "empty map",
			attrsFunc: pcommon.NewMap,
			hint:      HintAggregateMetricDouble,
			want:      false,
		},
		{
			name: "bad type",
			attrsFunc: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutBool(MappingHintsAttrKey, true)
				return m
			},
			hint: HintAggregateMetricDouble,
			want: false,
		},
		{
			name: "bad inner type",
			attrsFunc: func() pcommon.Map {
				m := pcommon.NewMap()
				s := m.PutEmptySlice(MappingHintsAttrKey)
				s.AppendEmpty().SetBool(true)
				return m
			},
			hint: HintAggregateMetricDouble,
			want: false,
		},
		{
			name: "hit",
			attrsFunc: func() pcommon.Map {
				m := pcommon.NewMap()
				s := m.PutEmptySlice(MappingHintsAttrKey)
				s.AppendEmpty().SetStr(string(HintAggregateMetricDouble))
				return m
			},
			hint: HintAggregateMetricDouble,
			want: true,
		},
		{
			name: "hit 2nd",
			attrsFunc: func() pcommon.Map {
				m := pcommon.NewMap()
				s := m.PutEmptySlice(MappingHintsAttrKey)
				s.AppendEmpty().SetStr(string(HintDocCount))
				s.AppendEmpty().SetStr(string(HintAggregateMetricDouble))
				return m
			},
			hint: HintAggregateMetricDouble,
			want: true,
		},
		{
			name: "miss",
			attrsFunc: func() pcommon.Map {
				m := pcommon.NewMap()
				s := m.PutEmptySlice(MappingHintsAttrKey)
				s.AppendEmpty().SetStr(string(HintDocCount))
				return m
			},
			hint: HintAggregateMetricDouble,
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, NewMappingHintGetter(tt.attrsFunc()).HasMappingHint(tt.hint))
		})
	}
}
