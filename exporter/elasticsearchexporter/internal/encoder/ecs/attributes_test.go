// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ecs

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/objmodel"
)

func TestEncoreAttributes(t *testing.T) {
	tests := map[string]struct {
		attrs         func() pcommon.Map
		conversionMap map[string]string
		expectedDoc   func() objmodel.Document
	}{
		"no_attrs": {
			attrs: pcommon.NewMap,
			conversionMap: map[string]string{
				"foo.bar": "baz",
			},
			expectedDoc: func() objmodel.Document {
				return objmodel.Document{}
			},
		},
		"no_conversion_map": {
			attrs: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("foo.bar", "baz")
				return m
			},
			expectedDoc: func() objmodel.Document {
				d := objmodel.Document{}
				d.AddString("foo.bar", "baz")
				return d
			},
		},
		"empty_conversion_map": {
			attrs: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("foo.bar", "baz")
				return m
			},
			conversionMap: map[string]string{},
			expectedDoc: func() objmodel.Document {
				d := objmodel.Document{}
				d.AddString("foo.bar", "baz")
				return d
			},
		},
		"all_attrs_in_conversion_map": {
			attrs: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("foo.bar", "baz")
				m.PutInt("qux", 17)
				return m
			},
			conversionMap: map[string]string{
				"foo.bar": "bar.qux",
				"qux":     "foo",
			},
			expectedDoc: func() objmodel.Document {
				d := objmodel.Document{}
				d.AddString("bar.qux", "baz")
				d.AddInt("foo", 17)
				return d
			},
		},
		"some_attrs_in_conversion_map": {
			attrs: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("foo.bar", "baz")
				m.PutInt("qux", 17)
				return m
			},
			conversionMap: map[string]string{
				"foo.bar": "bar.qux",
			},
			expectedDoc: func() objmodel.Document {
				d := objmodel.Document{}
				d.AddString("bar.qux", "baz")
				d.AddInt("qux", 17)
				return d
			},
		},
		"no_attrs_in_conversion_map": {
			attrs: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("foo.bar", "baz")
				m.PutInt("qux", 17)
				return m
			},
			conversionMap: map[string]string{
				"baz": "qux",
			},
			expectedDoc: func() objmodel.Document {
				d := objmodel.Document{}
				d.AddString("foo.bar", "baz")
				d.AddInt("qux", 17)
				return d
			},
		},
		"extra_keys_in_conversion_map": {
			attrs: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("foo.bar", "baz")
				return m
			},
			conversionMap: map[string]string{
				"foo.bar": "bar.qux",
				"qux":     "foo",
			},
			expectedDoc: func() objmodel.Document {
				d := objmodel.Document{}
				d.AddString("bar.qux", "baz")
				return d
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			var doc objmodel.Document
			EncodeAttributes(&doc, test.attrs(), test.conversionMap)

			expectedDoc := test.expectedDoc()
			require.Equal(t, expectedDoc, doc)
		})
	}
}
