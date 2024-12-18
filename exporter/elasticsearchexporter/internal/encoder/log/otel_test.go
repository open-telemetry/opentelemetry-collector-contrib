// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package log

import (
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/datastream"
)

// TODO: remove const repetition
const (
	maxDataStreamBytes       = 100
	disallowedNamespaceRunes = "\\/*?\"<>| ,#:"
	disallowedDatasetRunes   = "-\\/*?\"<>| ,#:"
)

func TestEncodeLogOtelMode(t *testing.T) {
	randomString := strings.Repeat("abcdefghijklmnopqrstuvwxyz0123456789", 10)
	maxLenNamespace := maxDataStreamBytes - len(disallowedNamespaceRunes)
	maxLenDataset := maxDataStreamBytes - len(disallowedDatasetRunes) - len(".otel")

	tests := []struct {
		name   string
		rec    OTelRecord
		wantFn func(OTelRecord) OTelRecord // Allows each test to customized the expectations from the original test record data
	}{
		{
			name: "default", // Expecting default data_stream values
			rec:  buildOTelRecordTestData(t, nil),
			wantFn: func(or OTelRecord) OTelRecord {
				return assignDatastreamData(or)
			},
		},
		{
			name: "custom dataset",
			rec: buildOTelRecordTestData(t, func(or OTelRecord) OTelRecord {
				or.Attributes["data_stream.dataset"] = "custom"
				return or
			}),
			wantFn: func(or OTelRecord) OTelRecord {
				// Datastream attributes are expected to be deleted from under the attributes
				deleteDatasetAttributes(or)
				return assignDatastreamData(or, "", "custom.otel")
			},
		},
		{
			name: "custom dataset with otel suffix",
			rec: buildOTelRecordTestData(t, func(or OTelRecord) OTelRecord {
				or.Attributes["data_stream.dataset"] = "custom.otel"
				return or
			}),
			wantFn: func(or OTelRecord) OTelRecord {
				deleteDatasetAttributes(or)
				return assignDatastreamData(or, "", "custom.otel.otel")
			},
		},
		{
			name: "custom dataset/namespace",
			rec: buildOTelRecordTestData(t, func(or OTelRecord) OTelRecord {
				or.Attributes["data_stream.dataset"] = "customds"
				or.Attributes["data_stream.namespace"] = "customns"
				return or
			}),
			wantFn: func(or OTelRecord) OTelRecord {
				deleteDatasetAttributes(or)
				return assignDatastreamData(or, "", "customds.otel", "customns")
			},
		},
		{
			name: "dataset attributes priority",
			rec: buildOTelRecordTestData(t, func(or OTelRecord) OTelRecord {
				or.Attributes["data_stream.dataset"] = "first"
				or.Scope.Attributes["data_stream.dataset"] = "second"
				or.Resource.Attributes["data_stream.dataset"] = "third"
				return or
			}),
			wantFn: func(or OTelRecord) OTelRecord {
				deleteDatasetAttributes(or)
				return assignDatastreamData(or, "", "first.otel")
			},
		},
		{
			name: "dataset scope attribute priority",
			rec: buildOTelRecordTestData(t, func(or OTelRecord) OTelRecord {
				or.Scope.Attributes["data_stream.dataset"] = "second"
				or.Resource.Attributes["data_stream.dataset"] = "third"
				return or
			}),
			wantFn: func(or OTelRecord) OTelRecord {
				deleteDatasetAttributes(or)
				return assignDatastreamData(or, "", "second.otel")
			},
		},
		{
			name: "dataset resource attribute priority",
			rec: buildOTelRecordTestData(t, func(or OTelRecord) OTelRecord {
				or.Resource.Attributes["data_stream.dataset"] = "third"
				return or
			}),
			wantFn: func(or OTelRecord) OTelRecord {
				deleteDatasetAttributes(or)
				return assignDatastreamData(or, "", "third.otel")
			},
		},
		{
			name: "sanitize dataset/namespace",
			rec: buildOTelRecordTestData(t, func(or OTelRecord) OTelRecord {
				or.Attributes["data_stream.dataset"] = disallowedDatasetRunes + randomString
				or.Attributes["data_stream.namespace"] = disallowedNamespaceRunes + randomString
				return or
			}),
			wantFn: func(or OTelRecord) OTelRecord {
				deleteDatasetAttributes(or)
				ds := strings.Repeat("_", len(disallowedDatasetRunes)) + randomString[:maxLenDataset] + ".otel"
				ns := strings.Repeat("_", len(disallowedNamespaceRunes)) + randomString[:maxLenNamespace]
				return assignDatastreamData(or, "", ds, ns)
			},
		},
	}

	e := &otelEncoder{true}
	for _, tc := range tests {
		record, scope, resource := createTestOTelLogRecord(t, tc.rec)

		// This sets the data_stream values default or derived from the record/scope/resources
		datastream.RouteLogRecord(record.Attributes(), scope.Attributes(), resource.Attributes(), "", true, scope.Name())

		b, err := e.EncodeLog(resource, tc.rec.Resource.SchemaURL, record, scope, tc.rec.Scope.SchemaURL)
		require.NoError(t, err)

		want := tc.rec
		if tc.wantFn != nil {
			want = tc.wantFn(want)
		}

		var got OTelRecord
		err = json.Unmarshal(b, &got)

		require.NoError(t, err)

		assert.Equal(t, want, got)
	}
}

// JSON serializable structs for OTel test convenience
type OTelRecord struct {
	TraceID                OTelTraceID          `json:"trace_id"`
	SpanID                 OTelSpanID           `json:"span_id"`
	Timestamp              time.Time            `json:"@timestamp"`
	ObservedTimestamp      time.Time            `json:"observed_timestamp"`
	SeverityNumber         int32                `json:"severity_number"`
	SeverityText           string               `json:"severity_text"`
	Attributes             map[string]any       `json:"attributes"`
	DroppedAttributesCount uint32               `json:"dropped_attributes_count"`
	Scope                  OTelScope            `json:"scope"`
	Resource               OTelResource         `json:"resource"`
	Datastream             OTelRecordDatastream `json:"data_stream"`
}

type OTelRecordDatastream struct {
	Dataset   string `json:"dataset"`
	Namespace string `json:"namespace"`
	Type      string `json:"type"`
}

type OTelScope struct {
	Name                   string         `json:"name"`
	Version                string         `json:"version"`
	Attributes             map[string]any `json:"attributes"`
	DroppedAttributesCount uint32         `json:"dropped_attributes_count"`
	SchemaURL              string         `json:"schema_url"`
}

type OTelResource struct {
	Attributes             map[string]any `json:"attributes"`
	DroppedAttributesCount uint32         `json:"dropped_attributes_count"`
	SchemaURL              string         `json:"schema_url"`
}

type OTelSpanID pcommon.SpanID

func (o OTelSpanID) MarshalJSON() ([]byte, error) {
	return nil, nil
}

func (o *OTelSpanID) UnmarshalJSON(data []byte) error {
	b, err := decodeOTelID(data)
	if err != nil {
		return err
	}
	copy(o[:], b)
	return nil
}

type OTelTraceID pcommon.TraceID

func (o OTelTraceID) MarshalJSON() ([]byte, error) {
	return nil, nil
}

func (o *OTelTraceID) UnmarshalJSON(data []byte) error {
	b, err := decodeOTelID(data)
	if err != nil {
		return err
	}
	copy(o[:], b)
	return nil
}

func decodeOTelID(data []byte) ([]byte, error) {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}

	return hex.DecodeString(s)
}

// helper function that creates the OTel LogRecord from the test structure
func createTestOTelLogRecord(t *testing.T, rec OTelRecord) (plog.LogRecord, pcommon.InstrumentationScope, pcommon.Resource) {
	record := plog.NewLogRecord()
	record.SetTimestamp(pcommon.Timestamp(uint64(rec.Timestamp.UnixNano())))                 //nolint:gosec // this input is controlled by tests
	record.SetObservedTimestamp(pcommon.Timestamp(uint64(rec.ObservedTimestamp.UnixNano()))) //nolint:gosec // this input is controlled by tests

	record.SetTraceID(pcommon.TraceID(rec.TraceID))
	record.SetSpanID(pcommon.SpanID(rec.SpanID))
	record.SetSeverityNumber(plog.SeverityNumber(rec.SeverityNumber))
	record.SetSeverityText(rec.SeverityText)
	record.SetDroppedAttributesCount(rec.DroppedAttributesCount)

	err := record.Attributes().FromRaw(rec.Attributes)
	require.NoError(t, err)

	scope := pcommon.NewInstrumentationScope()
	scope.SetName(rec.Scope.Name)
	scope.SetVersion(rec.Scope.Version)
	scope.SetDroppedAttributesCount(rec.Scope.DroppedAttributesCount)
	err = scope.Attributes().FromRaw(rec.Scope.Attributes)
	require.NoError(t, err)

	resource := pcommon.NewResource()
	resource.SetDroppedAttributesCount(rec.Resource.DroppedAttributesCount)
	err = resource.Attributes().FromRaw(rec.Resource.Attributes)
	require.NoError(t, err)

	return record, scope, resource
}

func buildOTelRecordTestData(t *testing.T, fn func(OTelRecord) OTelRecord) OTelRecord {
	s := `{
		"@timestamp": "2024-03-12T20:00:41.123456780Z",
		"attributes": {
				"event.name": "user-password-change",
				"foo.some": "bar"
		},
		"dropped_attributes_count": 1,
		"observed_timestamp": "2024-03-12T20:00:41.123456789Z",
		"resource": {
				"attributes": {
						"host.name": "lebuntu",
						"host.os.type": "linux"
				},
				"dropped_attributes_count": 2,
				"schema_url": "https://opentelemetry.io/schemas/1.6.0"
		},
		"scope": {
				"attributes": {
						"attr.num": 1234,
						"attr.str": "val1"
				},
				"dropped_attributes_count": 2,
				"name": "foobar",
				"schema_url": "https://opentelemetry.io/schemas/1.6.1",
				"version": "42"
		},
		"severity_number": 17,
		"severity_text": "ERROR",
		"span_id": "0102030405060708",
		"trace_id": "01020304050607080900010203040506"
}`

	var record OTelRecord
	err := json.Unmarshal([]byte(s), &record)
	assert.NoError(t, err)
	if fn != nil {
		record = fn(record)
	}
	return record
}

func deleteDatasetAttributes(or OTelRecord) {
	deleteDatasetAttributesFromMap(or.Attributes)
	deleteDatasetAttributesFromMap(or.Scope.Attributes)
	deleteDatasetAttributesFromMap(or.Resource.Attributes)
}

func deleteDatasetAttributesFromMap(m map[string]any) {
	delete(m, "data_stream.dataset")
	delete(m, "data_stream.namespace")
	delete(m, "data_stream.type")
}

func assignDatastreamData(or OTelRecord, a ...string) OTelRecord {
	r := OTelRecordDatastream{
		Dataset:   "generic.otel",
		Namespace: "default",
		Type:      "logs",
	}

	if len(a) > 0 && a[0] != "" {
		r.Type = a[0]
	}
	if len(a) > 1 && a[1] != "" {
		r.Dataset = a[1]
	}
	if len(a) > 2 && a[2] != "" {
		r.Namespace = a[2]
	}

	or.Datastream = r

	return or
}
