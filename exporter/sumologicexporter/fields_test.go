// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumologicexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sumologicexporter"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestFields(t *testing.T) {
	testcases := []struct {
		name     string
		fields   map[string]string
		expected string
	}{
		{
			name: "string",
			fields: map[string]string{
				"key1": "value1",
				"key3": "value3",
				"key2": "value2",
			},
			expected: "key1=value1, key2=value2, key3=value3",
		},
		{
			name: "sanitization",
			fields: map[string]string{
				"key1":   "value,1",
				"key3":   "value\n3",
				"key=,2": "valu,e=2",
			},
			expected: "key1=value_1, key3=value_3, key:_2=valu_e:2",
		},
		{
			name: "empty element",
			fields: map[string]string{
				"key1": "value1",
				"key3": "value3",
				"key2": "",
			},
			expected: "key1=value1, key3=value3",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			flds := fieldsFromMap(tc.fields)

			assert.Equal(t, tc.expected, flds.string())
		})
	}
}

func BenchmarkFields(b *testing.B) {
	attrMap := pcommon.NewMap()
	flds := map[string]any{
		"key1": "value1",
		"key3": "value3",
		"key2": "",
		"map": map[string]string{
			"key1": "value1",
			"key3": "value3",
			"key2": "",
		},
	}
	for k, v := range flds {
		switch v := v.(type) {
		case string:
			attrMap.PutStr(k, v)
		case map[string]string:
			m := pcommon.NewValueMap()
			mm := m.Map().AsRaw()
			for kk, vv := range v {
				mm[kk] = vv
			}
			m.CopyTo(attrMap.PutEmpty(k))
		}
	}
	sut := newFields(attrMap)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sut.string()
	}
}
