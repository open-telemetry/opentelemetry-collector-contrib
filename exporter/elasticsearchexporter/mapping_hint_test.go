// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestHasHint(t *testing.T) {
	tests := []struct {
		name      string
		attrsFunc func() pcommon.Map
		hint      mappingHint
		want      bool
	}{
		{
			name:      "empty map",
			attrsFunc: pcommon.NewMap,
			hint:      hintAggregateMetricDouble,
			want:      false,
		},
		{
			name: "bad type",
			attrsFunc: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutBool(mappingHintsAttrKey, true)
				return m
			},
			hint: hintAggregateMetricDouble,
			want: false,
		},
		{
			name: "bad inner type",
			attrsFunc: func() pcommon.Map {
				m := pcommon.NewMap()
				s := m.PutEmptySlice(mappingHintsAttrKey)
				s.AppendEmpty().SetBool(true)
				return m
			},
			hint: hintAggregateMetricDouble,
			want: false,
		},
		{
			name: "hit",
			attrsFunc: func() pcommon.Map {
				m := pcommon.NewMap()
				s := m.PutEmptySlice(mappingHintsAttrKey)
				s.AppendEmpty().SetStr(string(hintAggregateMetricDouble))
				return m
			},
			hint: hintAggregateMetricDouble,
			want: true,
		},
		{
			name: "hit 2nd",
			attrsFunc: func() pcommon.Map {
				m := pcommon.NewMap()
				s := m.PutEmptySlice(mappingHintsAttrKey)
				s.AppendEmpty().SetStr(string(hintDocCount))
				s.AppendEmpty().SetStr(string(hintAggregateMetricDouble))
				return m
			},
			hint: hintAggregateMetricDouble,
			want: true,
		},
		{
			name: "miss",
			attrsFunc: func() pcommon.Map {
				m := pcommon.NewMap()
				s := m.PutEmptySlice(mappingHintsAttrKey)
				s.AppendEmpty().SetStr(string(hintDocCount))
				return m
			},
			hint: hintAggregateMetricDouble,
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, newMappingHintGetter(tt.attrsFunc()).HasMappingHint(tt.hint))
		})
	}
}
