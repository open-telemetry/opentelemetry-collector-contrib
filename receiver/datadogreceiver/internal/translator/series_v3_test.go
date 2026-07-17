// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/DataDog/agent-payload/v5/gogen"
	intakev3 "github.com/DataDog/agent-payload/v5/metrics/intake_v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"google.golang.org/protobuf/proto"
)

// v3StringDict encodes strings into the {uvarint length, bytes} concatenation
// used by the v3 string dictionaries.
func v3StringDict(strs ...string) []byte {
	var out []byte
	for _, s := range strs {
		out = append(out, byte(len(s)))
		out = append(out, s...)
	}
	return out
}

// buildMinimalV3Payload constructs an intake_v3.Payload containing a single
// gauge metric "test.gauge" with tag "env:test", host resource "node-a",
// timestamp 1000 and value 42.0.
func buildMinimalV3Payload() *intakev3.Payload {
	return &intakev3.Payload{
		MetricData: &intakev3.MetricData{
			DictNameStr:        v3StringDict("test.gauge"),
			DictTagStr:         v3StringDict("env:test"),
			DictTagsets:        []int64{1, 1}, // one tagset: size=1, delta 1 -> dictTagStr[1]
			DictResourceStr:    v3StringDict("host", "node-a"),
			DictResourceLen:    []int64{1},
			DictResourceType:   []int64{1},
			DictResourceName:   []int64{2},
			Types:              []uint64{uint64(intakev3.MetricType_Gauge) | uint64(intakev3.ValueType_Float64)},
			NameRefs:           []int64{1},
			TagsetRefs:         []int64{1},
			ResourcesRefs:      []int64{1},
			SourceTypeNameRefs: []int64{0},
			OriginInfoRefs:     []int64{0},
			Intervals:          []uint64{10},
			NumPoints:          []uint64{1},
			Timestamps:         []int64{1000},
			ValsFloat64:        []float64{42.0},
		},
	}
}

func v3Request(t *testing.T, pl *intakev3.Payload) *http.Request {
	t.Helper()
	raw, err := proto.Marshal(pl)
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, "/api/intake/metrics/v3/series", bytes.NewReader(raw))
	require.NoError(t, err)
	return req
}

func TestHandleSeriesV3Payload_SingleGauge(t *testing.T) {
	mt := NewMetricsTranslator(component.BuildInfo{}, 0)
	series, err := mt.HandleSeriesV3Payload(v3Request(t, buildMinimalV3Payload()))
	require.NoError(t, err)
	require.Len(t, series, 1)

	s := series[0]
	assert.Equal(t, "test.gauge", s.Metric)
	assert.Equal(t, []string{"env:test"}, s.Tags)
	require.Len(t, s.Resources, 1)
	assert.Equal(t, "host", s.Resources[0].Type)
	assert.Equal(t, "node-a", s.Resources[0].Name)
	assert.Equal(t, gogen.MetricPayload_GAUGE, s.Type)
	assert.Empty(t, s.Unit)
	assert.Equal(t, int64(10), s.Interval)
	require.Len(t, s.Points, 1)
	assert.Equal(t, int64(1000), s.Points[0].Timestamp)
	assert.Equal(t, 42.0, s.Points[0].Value)
}

func TestHandleSeriesV3Payload_Unit(t *testing.T) {
	pl := buildMinimalV3Payload()
	pl.MetricData.Types[0] |= uint64(intakev3.MetricFlags_flagHasUnit)
	pl.MetricData.DictUnitStr = v3StringDict("millisecond")
	pl.MetricData.UnitRefs = []int64{1}

	mt := NewMetricsTranslator(component.BuildInfo{}, 0)
	series, err := mt.HandleSeriesV3Payload(v3Request(t, pl))
	require.NoError(t, err)
	require.Len(t, series, 1)
	assert.Equal(t, "millisecond", series[0].Unit)
}

// TestHandleSeriesV3Payload_DeltaEncoding exercises the delta encoded
// reference columns across several metrics, the payload-global timestamp
// accumulator, and the per-value-type columns including unstored zeros.
func TestHandleSeriesV3Payload_DeltaEncoding(t *testing.T) {
	pl := &intakev3.Payload{
		MetricData: &intakev3.MetricData{
			DictNameStr: v3StringDict("m.count", "m.rate"),
			DictTagStr:  v3StringDict("a:1", "b:2"),
			// tagset 1: size=2, deltas 1,1 -> ["a:1","b:2"]; tagset 2: size=1, delta 1 -> ["a:1"]
			DictTagsets:        []int64{2, 1, 1, 1, 1},
			DictResourceStr:    v3StringDict("host", "h1"),
			DictResourceLen:    []int64{1},
			DictResourceType:   []int64{1},
			DictResourceName:   []int64{2},
			DictSourceTypeName: v3StringDict("System"),
			DictUnitStr:        v3StringDict("second"),
			// unitRefs is a sparse column: only the second metric carries
			// flagHasUnit, so a single entry serves the whole payload
			UnitRefs: []int64{1},
			Types: []uint64{
				uint64(intakev3.MetricType_Count) | uint64(intakev3.ValueType_Sint64),
				uint64(intakev3.MetricType_Rate) | uint64(intakev3.ValueType_Zero) | uint64(intakev3.MetricFlags_flagHasUnit),
			},
			NameRefs:           []int64{1, 1},  // -> 1, 2
			TagsetRefs:         []int64{1, 1},  // -> 1, 2
			ResourcesRefs:      []int64{1, 0},  // -> 1, 1
			SourceTypeNameRefs: []int64{1, -1}, // -> 1, 0
			OriginInfoRefs:     []int64{0, 0},
			Intervals:          []uint64{10, 20},
			NumPoints:          []uint64{2, 1},
			// deltas accumulate across metric boundaries: 1000, 1010, 1005
			Timestamps: []int64{1000, 10, -5},
			ValsSint64: []int64{7, 9}, // zero values are not stored
		},
	}

	mt := NewMetricsTranslator(component.BuildInfo{}, 0)
	series, err := mt.HandleSeriesV3Payload(v3Request(t, pl))
	require.NoError(t, err)
	require.Len(t, series, 2)

	count := series[0]
	assert.Equal(t, "m.count", count.Metric)
	assert.Equal(t, []string{"a:1", "b:2"}, count.Tags)
	assert.Equal(t, gogen.MetricPayload_COUNT, count.Type)
	assert.Empty(t, count.Unit) // no flagHasUnit: the sparse unit column is not consumed
	assert.Equal(t, "System", count.SourceTypeName)
	require.Len(t, count.Points, 2)
	assert.Equal(t, int64(1000), count.Points[0].Timestamp)
	assert.Equal(t, 7.0, count.Points[0].Value)
	assert.Equal(t, int64(1010), count.Points[1].Timestamp)
	assert.Equal(t, 9.0, count.Points[1].Value)

	rate := series[1]
	assert.Equal(t, "m.rate", rate.Metric)
	assert.Equal(t, []string{"a:1"}, rate.Tags)
	assert.Equal(t, gogen.MetricPayload_RATE, rate.Type)
	assert.Equal(t, "second", rate.Unit)
	assert.Empty(t, rate.SourceTypeName)
	assert.Equal(t, int64(20), rate.Interval)
	require.Len(t, rate.Resources, 1)
	assert.Equal(t, "h1", rate.Resources[0].Name)
	require.Len(t, rate.Points, 1)
	assert.Equal(t, int64(1005), rate.Points[0].Timestamp)
	assert.Equal(t, 0.0, rate.Points[0].Value)
}

// TestHandleSeriesV3Payload_TagsetSpliceAndMetadata exercises negative tagset
// references (splicing a previously decoded tagset) and the union of
// payload-level metadata tags and resources.
func TestHandleSeriesV3Payload_TagsetSpliceAndMetadata(t *testing.T) {
	pl := &intakev3.Payload{
		Metadata: &intakev3.Metadata{
			Tags:      []string{"team:core"},
			Resources: []string{"cloud", "aws"},
		},
		MetricData: &intakev3.MetricData{
			DictNameStr: v3StringDict("m.one", "m.two"),
			DictTagStr:  v3StringDict("x:1", "y:2"),
			// tagset 1: size=2 -> ["x:1","y:2"]
			// tagset 2: size=2, deltas -1,2 -> splice(tagset 1) + idx 1 -> ["x:1","y:2","x:1"]
			DictTagsets:     []int64{2, 1, 1, 2, -1, 2},
			DictResourceLen: []int64{0}, // one empty resource set; metadata resources still apply
			Types: []uint64{
				uint64(intakev3.MetricType_Gauge) | uint64(intakev3.ValueType_Float64),
				uint64(intakev3.MetricType_Gauge) | uint64(intakev3.ValueType_Float64),
			},
			NameRefs:           []int64{1, 1},
			TagsetRefs:         []int64{1, 1},
			ResourcesRefs:      []int64{1, -1}, // deltas -> 1, 0: second metric has no resource set
			SourceTypeNameRefs: []int64{0, 0},
			OriginInfoRefs:     []int64{0, 0},
			Intervals:          []uint64{0, 0},
			NumPoints:          []uint64{1, 1},
			Timestamps:         []int64{1000, 1},
			ValsFloat64:        []float64{1.0, 2.0},
		},
	}

	mt := NewMetricsTranslator(component.BuildInfo{}, 0)
	series, err := mt.HandleSeriesV3Payload(v3Request(t, pl))
	require.NoError(t, err)
	require.Len(t, series, 2)

	assert.Equal(t, []string{"x:1", "y:2", "team:core"}, series[0].Tags)
	require.Len(t, series[0].Resources, 1)
	assert.Equal(t, "cloud", series[0].Resources[0].Type)
	assert.Equal(t, "aws", series[0].Resources[0].Name)

	assert.Equal(t, []string{"x:1", "y:2", "x:1", "team:core"}, series[1].Tags)
	assert.Empty(t, series[1].Resources) // resource set 0 is the empty element
}

func TestHandleSeriesV3Payload_SketchInSeriesPayloadErrors(t *testing.T) {
	pl := &intakev3.Payload{
		MetricData: &intakev3.MetricData{
			DictNameStr:        v3StringDict("test.dist1"),
			DictTagsets:        []int64{0},
			Types:              []uint64{uint64(intakev3.MetricType_Sketch) | uint64(intakev3.ValueType_Zero)},
			NameRefs:           []int64{1},
			TagsetRefs:         []int64{0},
			ResourcesRefs:      []int64{0},
			SourceTypeNameRefs: []int64{0},
			OriginInfoRefs:     []int64{0},
			Intervals:          []uint64{0},
			NumPoints:          []uint64{1},
			Timestamps:         []int64{1000},
			SketchNumBins:      []uint64{0},
			ValsSint64:         []int64{0},
		},
	}

	mt := NewMetricsTranslator(component.BuildInfo{}, 0)
	_, err := mt.HandleSeriesV3Payload(v3Request(t, pl))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected sketch metric")
}

func TestHandleSeriesV3Payload_Empty(t *testing.T) {
	mt := NewMetricsTranslator(component.BuildInfo{}, 0)

	// payload without metric data
	series, err := mt.HandleSeriesV3Payload(v3Request(t, &intakev3.Payload{}))
	require.NoError(t, err)
	assert.Empty(t, series)

	// empty body
	req, err := http.NewRequest(http.MethodPost, "/api/intake/metrics/v3/series", bytes.NewReader(nil))
	require.NoError(t, err)
	series, err = mt.HandleSeriesV3Payload(req)
	require.NoError(t, err)
	assert.Empty(t, series)
}

func TestHandleSeriesV3Payload_Malformed(t *testing.T) {
	mt := NewMetricsTranslator(component.BuildInfo{}, 0)

	for name, mutate := range map[string]func(*intakev3.MetricData){
		"truncated name dictionary": func(md *intakev3.MetricData) {
			md.DictNameStr = []byte{10, 'a'} // declared length exceeds data
		},
		"name reference out of range": func(md *intakev3.MetricData) {
			md.NameRefs = []int64{5}
		},
		"negative tagset reference": func(md *intakev3.MetricData) {
			md.TagsetRefs = []int64{-1}
		},
		"reference columns shorter than types": func(md *intakev3.MetricData) {
			md.NameRefs = nil
		},
		"points exceed timestamps column": func(md *intakev3.MetricData) {
			md.NumPoints = []uint64{2}
		},
		"points exceed value column": func(md *intakev3.MetricData) {
			md.NumPoints = []uint64{2}
			md.Timestamps = []int64{1000, 10}
		},
		"invalid UTF-8 in name dictionary": func(md *intakev3.MetricData) {
			md.DictNameStr = []byte{2, 0xff, 0xfe}
		},
		"tagset size exceeds column": func(md *intakev3.MetricData) {
			md.DictTagsets = []int64{5, 1}
		},
	} {
		t.Run(name, func(t *testing.T) {
			pl := buildMinimalV3Payload()
			mutate(pl.MetricData)
			_, err := mt.HandleSeriesV3Payload(v3Request(t, pl))
			require.Error(t, err)
		})
	}
}

func TestHandleSeriesV3Payload_InvalidUTF8TagSanitized(t *testing.T) {
	pl := buildMinimalV3Payload()
	pl.MetricData.DictTagStr = []byte{2, 0xff, 0xfe} // invalid UTF-8 tag string

	mt := NewMetricsTranslator(component.BuildInfo{}, 0)
	series, err := mt.HandleSeriesV3Payload(v3Request(t, pl))
	require.NoError(t, err)
	require.Len(t, series, 1)
	require.Len(t, series[0].Tags, 1)
	assert.NotEmpty(t, series[0].Tags[0])
}
