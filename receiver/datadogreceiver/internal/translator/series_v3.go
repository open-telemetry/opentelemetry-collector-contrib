// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver/internal/translator"

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/DataDog/agent-payload/v5/gogen"
	intakev3 "github.com/DataDog/agent-payload/v5/metrics/intake_v3"
	"google.golang.org/protobuf/proto"
)

// The v3 series intake (/api/intake/metrics/v3/series) is the default series
// endpoint for Datadog Agent 7.81.0 and later. Unlike the row-based
// MetricPayload of the v2 endpoint, it is a columnar, dictionary-encoded
// protobuf format (agent-payload metrics/intake_v3): strings are interned in
// dictionaries, per-metric dictionary references and point timestamps are
// delta encoded, and point values are stored in per-value-type columns. The
// decoder below follows the reference decoder in the datadog-agent fakeintake
// (test/fakeintake/aggregator/metricReaderV3.go) and converts a v3 payload to
// the v2 series representation so the translation to OTLP is shared with the
// v2 endpoint.

var (
	errV3UnexpectedEOF = errors.New("v3 payload: unexpected end of column")
	errV3Overflow      = errors.New("v3 payload: length field overflow")
	errV3BadReference  = errors.New("v3 payload: invalid dictionary reference")
	errV3InvalidUTF8   = errors.New("v3 payload: invalid UTF-8 string")
)

// HandleSeriesV3Payload parses a v3 series intake request into v2 metric
// series. The request body is always a protobuf-encoded intake_v3.Payload;
// any Content-Encoding is decompressed by confighttp before the handler runs.
func (*MetricsTranslator) HandleSeriesV3Payload(req *http.Request) ([]*gogen.MetricPayload_MetricSeries, error) {
	buf := GetBuffer()
	defer PutBuffer(buf)
	if _, err := io.Copy(buf, req.Body); err != nil {
		return nil, err
	}

	pl := new(intakev3.Payload)
	if err := proto.Unmarshal(buf.Bytes(), pl); err != nil {
		return nil, err
	}
	return translateV3Series(pl)
}

func translateV3Series(pl *intakev3.Payload) ([]*gogen.MetricPayload_MetricSeries, error) {
	if pl.GetMetricData() == nil {
		return nil, nil
	}
	d, err := newSeriesV3Decoder(pl)
	if err != nil {
		return nil, err
	}
	return d.decode()
}

// seriesV3Decoder walks the columnar MetricData. Dictionaries are base-1
// indexed with an implicit empty element at index zero. The per-metric
// reference columns (nameRefs, tagsetRefs, ...) are delta encoded across the
// whole payload, as are point timestamps; the unitRefs column is sparse and
// only advances for metrics carrying flagHasUnit.
type seriesV3Decoder struct {
	md *intakev3.MetricData

	// dictionaries, pre-unpacked; index zero is the empty element
	dictName           []string
	dictUnit           []string
	dictTagsets        [][]string
	dictResources      [][][2]string
	dictSourceTypeName []string
	originInfoCount    int64

	// delta accumulators for the per-metric reference columns
	nameRef           int64
	tagsetRef         int64
	resourcesRef      int64
	sourceTypeNameRef int64
	originInfoRef     int64
	unitRef           int64
	unitRefIdx        int

	// point cursors
	timestamp   int64 // delta accumulated across all points of the payload
	pointIdx    int
	valsSint64  int
	valsFloat32 int
	valsFloat64 int
}

func newSeriesV3Decoder(pl *intakev3.Payload) (*seriesV3Decoder, error) {
	md := pl.GetMetricData()
	d := &seriesV3Decoder{md: md}

	var err error
	if d.dictName, err = unpackV3StringDict(md.DictNameStr, false); err != nil {
		return nil, err
	}
	dictTagsStr, err := unpackV3StringDict(md.DictTagStr, true)
	if err != nil {
		return nil, err
	}
	if d.dictUnit, err = unpackV3StringDict(md.DictUnitStr, false); err != nil {
		return nil, err
	}
	if d.dictTagsets, err = unpackV3Tagsets(md.DictTagsets, dictTagsStr, pl.GetMetadata().GetTags()); err != nil {
		return nil, err
	}
	dictResourceStr, err := unpackV3StringDict(md.DictResourceStr, false)
	if err != nil {
		return nil, err
	}
	if d.dictResources, err = unpackV3Resources(md, dictResourceStr, pl.GetMetadata().GetResources()); err != nil {
		return nil, err
	}
	if d.dictSourceTypeName, err = unpackV3StringDict(md.DictSourceTypeName, false); err != nil {
		return nil, err
	}
	if len(md.DictOriginInfo)%3 != 0 {
		return nil, errV3UnexpectedEOF
	}
	// Origin info has no v2 equivalent; only the dictionary size is needed to
	// validate the originInfoRefs column.
	d.originInfoCount = int64(len(md.DictOriginInfo)/3) + 1

	return d, nil
}

func (d *seriesV3Decoder) decode() ([]*gogen.MetricPayload_MetricSeries, error) {
	md := d.md
	n := len(md.Types)
	if len(md.NameRefs) < n || len(md.TagsetRefs) < n || len(md.ResourcesRefs) < n ||
		len(md.SourceTypeNameRefs) < n || len(md.OriginInfoRefs) < n ||
		len(md.Intervals) < n || len(md.NumPoints) < n {
		return nil, errV3UnexpectedEOF
	}

	series := make([]*gogen.MetricPayload_MetricSeries, 0, n)
	for i := range n {
		packed := md.Types[i]
		metricType := intakev3.MetricType(packed & 0xF)
		valueType := intakev3.ValueType(packed & 0xF0)

		d.nameRef += md.NameRefs[i]
		if d.nameRef < 0 || d.nameRef >= int64(len(d.dictName)) {
			return nil, errV3BadReference
		}
		d.tagsetRef += md.TagsetRefs[i]
		if d.tagsetRef < 0 || d.tagsetRef >= int64(len(d.dictTagsets)) {
			return nil, errV3BadReference
		}
		d.resourcesRef += md.ResourcesRefs[i]
		if d.resourcesRef < 0 || d.resourcesRef >= int64(len(d.dictResources)) {
			return nil, errV3BadReference
		}
		d.sourceTypeNameRef += md.SourceTypeNameRefs[i]
		if d.sourceTypeNameRef < 0 || d.sourceTypeNameRef >= int64(len(d.dictSourceTypeName)) {
			return nil, errV3BadReference
		}
		d.originInfoRef += md.OriginInfoRefs[i]
		if d.originInfoRef < 0 || d.originInfoRef >= d.originInfoCount {
			return nil, errV3BadReference
		}
		var unit string
		if packed&uint64(intakev3.MetricFlags_flagHasUnit) != 0 {
			d.unitRefIdx++
			if d.unitRefIdx > len(md.UnitRefs) {
				return nil, errV3UnexpectedEOF
			}
			d.unitRef += md.UnitRefs[d.unitRefIdx-1]
			if d.unitRef < 0 || d.unitRef >= int64(len(d.dictUnit)) {
				return nil, errV3BadReference
			}
			unit = d.dictUnit[d.unitRef]
		}

		var seriesType gogen.MetricPayload_MetricType
		switch metricType {
		case intakev3.MetricType_Count:
			seriesType = gogen.MetricPayload_COUNT
		case intakev3.MetricType_Rate:
			seriesType = gogen.MetricPayload_RATE
		case intakev3.MetricType_Gauge:
			seriesType = gogen.MetricPayload_GAUGE
		case intakev3.MetricType_Sketch:
			// The agent submits sketches to /api/intake/metrics/v3/sketches,
			// never to the series endpoint.
			return nil, fmt.Errorf("v3 payload: unexpected sketch metric %q in series payload", d.dictName[d.nameRef])
		default:
			// Unknown types are dropped by TranslateSeriesV2, but their points
			// must still be consumed to keep the value cursors aligned.
			seriesType = gogen.MetricPayload_UNSPECIFIED
		}

		if md.Intervals[i] > math.MaxInt64 {
			return nil, errV3Overflow
		}

		numPoints := md.NumPoints[i]
		if numPoints > uint64(len(md.Timestamps)) {
			return nil, errV3UnexpectedEOF
		}
		points := make([]*gogen.MetricPayload_MetricPoint, 0, numPoints)
		for range numPoints {
			d.pointIdx++
			if d.pointIdx > len(md.Timestamps) {
				return nil, errV3UnexpectedEOF
			}
			d.timestamp += md.Timestamps[d.pointIdx-1]

			var val float64
			switch valueType {
			case intakev3.ValueType_Float64:
				d.valsFloat64++
				if d.valsFloat64 > len(md.ValsFloat64) {
					return nil, errV3UnexpectedEOF
				}
				val = md.ValsFloat64[d.valsFloat64-1]
			case intakev3.ValueType_Float32:
				d.valsFloat32++
				if d.valsFloat32 > len(md.ValsFloat32) {
					return nil, errV3UnexpectedEOF
				}
				val = float64(md.ValsFloat32[d.valsFloat32-1])
			case intakev3.ValueType_Sint64:
				d.valsSint64++
				if d.valsSint64 > len(md.ValsSint64) {
					return nil, errV3UnexpectedEOF
				}
				val = float64(md.ValsSint64[d.valsSint64-1])
			case intakev3.ValueType_Zero:
				// zero values are not stored in any column
			default:
				return nil, fmt.Errorf("v3 payload: unknown value type %#x", uint64(valueType))
			}
			points = append(points, &gogen.MetricPayload_MetricPoint{Timestamp: d.timestamp, Value: val})
		}

		serie := &gogen.MetricPayload_MetricSeries{
			Metric: d.dictName[d.nameRef],
			// tagsets are shared between series; hand each series its own copy
			Tags:           slices.Clone(d.dictTagsets[d.tagsetRef]),
			Type:           seriesType,
			Unit:           unit,
			SourceTypeName: d.dictSourceTypeName[d.sourceTypeNameRef],
			Interval:       int64(md.Intervals[i]),
			Points:         points,
		}
		for _, res := range d.dictResources[d.resourcesRef] {
			serie.Resources = append(serie.Resources, &gogen.MetricPayload_Resource{Type: res[0], Name: res[1]})
		}
		series = append(series, serie)
	}
	return series, nil
}

// unpackV3StringDict decodes a string dictionary: a concatenation of
// {uvarint length, bytes} entries, returned base-1 with the empty string at
// index zero. Tag strings tolerate invalid UTF-8 by sanitizing; all other
// dictionaries reject it.
func unpackV3StringDict(raw []byte, sanitizeInvalidUTF8 bool) ([]string, error) {
	dict := []string{""}
	for len(raw) > 0 {
		length, n := binary.Uvarint(raw)
		if n == 0 {
			return nil, errV3UnexpectedEOF
		}
		if n < 0 {
			return nil, errV3Overflow
		}
		if length > uint64(math.MaxInt-n) {
			return nil, errV3Overflow
		}
		end := n + int(length)
		if end > len(raw) {
			return nil, errV3UnexpectedEOF
		}
		str := string(raw[n:end])
		if !utf8.ValidString(str) {
			if !sanitizeInvalidUTF8 {
				return nil, errV3InvalidUTF8
			}
			str = strings.ToValidUTF8(str, string(utf8.RuneError))
		}
		dict = append(dict, str)
		raw = raw[end:]
	}
	return dict, nil
}

// unpackV3Tagsets decodes the tagset dictionary: a stream of
// {size, size x delta} groups over indexes into the tag string dictionary,
// with the delta accumulator resetting per tagset. A negative accumulated
// index splices in a previously decoded tagset. Payload-level metadata tags
// are unioned into every tagset.
func unpackV3Tagsets(packed []int64, dictTagsStr, metadataTags []string) ([][]string, error) {
	tagsets := [][]string{nil}

	for len(packed) > 0 {
		size := packed[0]
		packed = packed[1:]
		if size < 0 || size > int64(len(packed)) {
			return nil, errV3UnexpectedEOF
		}
		tags := make([]string, 0, int(size)+len(metadataTags))
		idx := int64(0)
		for i := range size {
			idx += packed[i]
			if idx < 0 {
				if idx <= -math.MaxInt64 || -idx >= int64(len(tagsets)) {
					return nil, errV3BadReference
				}
				tags = append(tags, tagsets[-idx]...)
			} else {
				if idx >= int64(len(dictTagsStr)) {
					return nil, errV3BadReference
				}
				tags = append(tags, dictTagsStr[idx])
			}
		}
		packed = packed[size:]
		tagsets = append(tagsets, tags)
	}

	if len(metadataTags) == 0 {
		return tagsets, nil
	}
	metaIndex := make(map[string]int, len(metadataTags))
	for i, mt := range metadataTags {
		metaIndex[mt] = i
	}
	for i, tags := range tagsets {
		if len(tags) == 0 {
			tagsets[i] = append(tags, metadataTags...)
			continue
		}
		seen := make([]bool, len(metadataTags))
		for _, t := range tags {
			if idx, ok := metaIndex[t]; ok {
				seen[idx] = true
			}
		}
		for idx, mt := range metadataTags {
			if !seen[idx] {
				tags = append(tags, mt)
			}
		}
		tagsets[i] = tags
	}
	return tagsets, nil
}

// unpackV3Resources decodes the resource-set dictionary: dictResourceLen holds
// the pair count of each set, and dictResourceType/dictResourceName are
// per-set delta encoded indexes into the resource string dictionary. Each
// entry is a [type, name] pair; payload-level metadata resources (a flat
// [type, name, type, name, ...] list) are appended to every set.
func unpackV3Resources(md *intakev3.MetricData, dictResourceStr, metadataResources []string) ([][][2]string, error) {
	packedType := md.DictResourceType
	packedName := md.DictResourceName
	dict := make([][][2]string, 1, len(md.DictResourceLen)+1)

	var metaResources [][2]string
	if len(metadataResources) > 0 {
		if len(metadataResources)%2 != 0 {
			return nil, errors.New("v3 payload: metadata resources must be [type, name] pairs")
		}
		metaResources = make([][2]string, 0, len(metadataResources)/2)
		for i := 0; i+1 < len(metadataResources); i += 2 {
			metaResources = append(metaResources, [2]string{metadataResources[i], metadataResources[i+1]})
		}
	}

	start := int64(0)
	for _, size := range md.DictResourceLen {
		if size < 0 {
			return nil, errV3UnexpectedEOF
		}
		if size > math.MaxInt64-start {
			return nil, errV3Overflow
		}
		end := start + size
		if end > int64(len(packedType)) || end > int64(len(packedName)) {
			return nil, errV3BadReference
		}
		typeRef, nameRef := int64(0), int64(0)
		set := make([][2]string, 0, size+int64(len(metaResources)))
		for i := range size {
			typeRef += packedType[start+i]
			nameRef += packedName[start+i]
			if typeRef < 0 || typeRef >= int64(len(dictResourceStr)) ||
				nameRef < 0 || nameRef >= int64(len(dictResourceStr)) {
				return nil, errV3BadReference
			}
			set = append(set, [2]string{dictResourceStr[typeRef], dictResourceStr[nameRef]})
		}
		set = append(set, metaResources...)
		dict = append(dict, set)
		start = end
	}
	return dict, nil
}
