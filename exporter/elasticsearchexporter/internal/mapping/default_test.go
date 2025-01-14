// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mapping

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/objmodel"
)

func TestEncodeAttributes(t *testing.T) {
	t.Parallel()

	attributes := pcommon.NewMap()
	err := attributes.FromRaw(map[string]any{
		"s": "baz",
		"o": map[string]any{
			"sub_i": 19,
		},
	})
	require.NoError(t, err)

	tests := map[string]struct {
		mappingMode Mode
		want        func() objmodel.Document
	}{
		"raw": {
			mappingMode: ModeRaw,
			want: func() objmodel.Document {
				return objmodel.DocumentFromAttributes(attributes)
			},
		},
		"none": {
			mappingMode: ModeNone,
			want: func() objmodel.Document {
				doc := objmodel.Document{}
				doc.AddAttributes("Attributes", attributes)
				return doc
			},
		},
		"ecs": {
			mappingMode: ModeECS,
			want: func() objmodel.Document {
				doc := objmodel.Document{}
				doc.AddAttributes("Attributes", attributes)
				return doc
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			doc := objmodel.Document{}
			encodeAttributes(&doc, test.mappingMode, attributes)
			require.Equal(t, test.want(), doc)
		})
	}
}

func TestEncodeEvents(t *testing.T) {
	t.Parallel()

	events := ptrace.NewSpanEventSlice()
	events.EnsureCapacity(4)
	for i := 0; i < 4; i++ {
		event := events.AppendEmpty()
		event.SetTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(time.Duration(i) * time.Minute)))
		event.SetName(fmt.Sprintf("event_%d", i))
	}

	tests := map[string]struct {
		mappingMode Mode
		want        func() objmodel.Document
	}{
		"raw": {
			mappingMode: ModeRaw,
			want: func() objmodel.Document {
				doc := objmodel.Document{}
				doc.AddEvents("", events)
				return doc
			},
		},
		"none": {
			mappingMode: ModeNone,
			want: func() objmodel.Document {
				doc := objmodel.Document{}
				doc.AddEvents("Events", events)
				return doc
			},
		},
		"ecs": {
			mappingMode: ModeECS,
			want: func() objmodel.Document {
				doc := objmodel.Document{}
				doc.AddEvents("Events", events)
				return doc
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			doc := objmodel.Document{}
			encodeEvents(&doc, test.mappingMode, events)
			require.Equal(t, test.want(), doc)
		})
	}
}
