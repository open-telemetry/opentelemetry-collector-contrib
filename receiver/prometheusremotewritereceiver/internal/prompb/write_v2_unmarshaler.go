package prompb

import (
	"fmt"
	"math"
	"sync"

	"github.com/VictoriaMetrics/easyproto"
	"github.com/prometheus/prometheus/model/labels"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
)

// GetWriteV2Unmarshaler returns WriteV2Unmarshaler from the pool.
//
// Return the WriteV2Unmarshaler to the pool when it is no longer needed via PutWriteV2Unmarshaler call.
func GetWriteV2Unmarshaler() *WriteV2Unmarshaler {
	v := wv2uPool.Get()
	if v == nil {
		return &WriteV2Unmarshaler{}
	}
	return v.(*WriteV2Unmarshaler)
}

// PutWriteV2Unmarshaler returns wru to the pool.
//
// The caller mustn't access wru fields after returning wru to the pool.
func PutWriteV2Unmarshaler(wru *WriteV2Unmarshaler) {
	wru.Reset()
	wv2uPool.Put(wru)
}

var wv2uPool sync.Pool

// WriteV2Request is a lightweight representation of Prometheus remote write v2 Request.
// It covers the commonly used fields: Symbols and Timeseries.
type WriteV2Request struct {
	Symbols    []string
	Timeseries []WriteV2TimeSeries
}

// WriteV2TimeSeries represents a single series in write v2.
type WriteV2TimeSeries struct {
	LabelsRefs []uint32
	Samples    []WriteV2Sample
	Histograms []WriteV2Histogram
	Exemplars  []WriteV2Exemplar
	// Metadata and other fields are available but optional for now.
	Metadata *WriteV2Metadata
}

// ToLabels return model labels.Labels from timeseries' remote labels.
func (m WriteV2TimeSeries) ToLabels(b *labels.ScratchBuilder, symbols []string) (labels.Labels, error) {
	return desymbolizeLabels(b, m.LabelsRefs, symbols)
}

// WriteV2Sample represents a sample in write v2.
type WriteV2Sample struct {
	Value          float64
	Timestamp      int64
	StartTimestamp int64
}

// WriteV2Histogram is a simplified representation of the native histogram message.
// It covers both integer and float variants and the bucket spans/counts needed for conversion.
type WriteV2Histogram struct {
	// count can be represented either as integer or float. We keep both and use presence checks.
	CountInt      uint64
	CountFloat    float64
	HasCountFloat bool

	Sum               float64
	Schema            int32
	ZeroThreshold     float64
	ZeroCountInt      uint64
	ZeroCountFloat    float64
	HasZeroCountFloat bool

	NegativeSpans  []BucketSpan
	NegativeDeltas []int64
	NegativeCounts []float64
	PositiveSpans  []BucketSpan
	PositiveDeltas []int64
	PositiveCounts []float64
	ResetHint      writev2.Histogram_ResetHint
	Timestamp      int64
	CustomValues   []float64
	StartTimestamp int64
}

// BucketSpan represents a span of buckets (offset and length).
type BucketSpan struct {
	Offset int32
	Length uint32
}

// WriteV2Exemplar represents an exemplar attached to samples.
type WriteV2Exemplar struct {
	LabelsRefs []uint32
	Value      float64
	Timestamp  int64
}

// WriteV2Metadata is a minimal representation of the v2 Metadata message.
type WriteV2Metadata struct {
	Type    writev2.Metadata_MetricType
	HelpRef uint32
	UnitRef uint32
}

// WriteV2Unmarshaler parses Protobuf-encoded write v2 Request messages.
// It reuses internal slices to avoid allocations between calls.
type WriteV2Unmarshaler struct {
	wr WriteV2Request

	// pools to reuse memory
	symbolsPool    []string
	timeseriesPool []WriteV2TimeSeries
	samplesPool    []WriteV2Sample
	histPool       []WriteV2Histogram
	exemplarPool   []WriteV2Exemplar
}

// Reset resets unmarshaler so it can be reused.
func (wru *WriteV2Unmarshaler) Reset() {
	wru.wr.Symbols = ResetStrings(wru.wr.Symbols)
	wru.wr.Timeseries = ResetWriteV2TimeSeries(wru.wr.Timeseries)

	// clear pools
	clearSlice(wru.symbolsPool)
	wru.symbolsPool = wru.symbolsPool[:0]
	clearSlice(wru.timeseriesPool)
	wru.timeseriesPool = wru.timeseriesPool[:0]
	clearSlice(wru.samplesPool)
	wru.samplesPool = wru.samplesPool[:0]
	clearSlice(wru.histPool)
	wru.histPool = wru.histPool[:0]
	clearSlice(wru.exemplarPool)
	wru.exemplarPool = wru.exemplarPool[:0]
}

// UnmarshalProtobuf parses src into an internal WriteV2Request and returns it.
// The returned pointer is valid until the next call to UnmarshalProtobuf on the same unmarshaler.
func (wru *WriteV2Unmarshaler) UnmarshalProtobuf(src []byte) (*WriteV2Request, error) {
	wru.Reset()

	var err error
	var fc easyproto.FieldContext

	symbols := wru.wr.Symbols
	ts := wru.wr.Timeseries
	symbolsPool := wru.symbolsPool
	timeseriesPool := wru.timeseriesPool
	samplesPool := wru.samplesPool
	histPool := wru.histPool
	exemplarPool := wru.exemplarPool

	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return nil, fmt.Errorf("cannot read the next field: %w", err)
		}
		switch fc.FieldNum {
		case 4: // symbols repeated string
			v, ok := fc.String()
			if !ok {
				return nil, fmt.Errorf("cannot read symbol string")
			}
			if len(symbols) < cap(symbols) {
				symbols = symbols[:len(symbols)+1]
			} else {
				symbols = append(symbols, "")
			}
			symbols[len(symbols)-1] = v
			// also keep in symbolsPool for reuse
			if len(symbolsPool) < cap(symbolsPool) {
				symbolsPool = symbolsPool[:len(symbolsPool)+1]
			} else {
				symbolsPool = append(symbolsPool, "")
			}
			symbolsPool[len(symbolsPool)-1] = v
		case 5: // timeseries repeated message
			data, ok := fc.MessageData()
			if !ok {
				return nil, fmt.Errorf("cannot read timeseries data")
			}
			// grow ts slice
			if len(ts) < cap(ts) {
				ts = ts[:len(ts)+1]
			} else {
				ts = append(ts, WriteV2TimeSeries{})
			}
			tsPtr := &ts[len(ts)-1]
			timeseriesPool, samplesPool, histPool, exemplarPool, err = unmarshalWriteV2TimeSeries(data, timeseriesPool, samplesPool, histPool, exemplarPool, tsPtr)
			if err != nil {
				return nil, fmt.Errorf("cannot unmarshal timeseries: %w", err)
			}
		}
	}

	wru.wr.Symbols = symbols
	wru.wr.Timeseries = ts
	wru.symbolsPool = symbolsPool
	wru.timeseriesPool = timeseriesPool
	wru.samplesPool = samplesPool
	wru.histPool = histPool
	wru.exemplarPool = exemplarPool
	return &wru.wr, nil
}

func unmarshalWriteV2TimeSeries(src []byte, tsPool []WriteV2TimeSeries, samplesPool []WriteV2Sample, histPool []WriteV2Histogram, exemplarPool []WriteV2Exemplar, out *WriteV2TimeSeries) ([]WriteV2TimeSeries, []WriteV2Sample, []WriteV2Histogram, []WriteV2Exemplar, error) {
	var fc easyproto.FieldContext
	for len(src) > 0 {
		var err error
		src, err = fc.NextField(src)
		if err != nil {
			return tsPool, samplesPool, histPool, exemplarPool, fmt.Errorf("cannot read the next field: %w", err)
		}
		switch fc.FieldNum {
		case 1: // labels_refs packed uint32
			// read packed varints
			vals, ok := fc.UnpackUint32s(out.LabelsRefs[:0])
			if !ok {
				// if not packed, try single value
				v, ok2 := fc.Uint32()
				if !ok2 {
					return tsPool, samplesPool, histPool, exemplarPool, fmt.Errorf("cannot read labels_refs")
				}
				out.LabelsRefs = append(out.LabelsRefs, v)
			} else {
				out.LabelsRefs = append(out.LabelsRefs, vals...)
			}
		case 2: // samples repeated message
			data, ok := fc.MessageData()
			if !ok {
				return tsPool, samplesPool, histPool, exemplarPool, fmt.Errorf("cannot read sample data")
			}
			// append sample from pool
			if len(samplesPool) < cap(samplesPool) {
				samplesPool = samplesPool[:len(samplesPool)+1]
			} else {
				samplesPool = append(samplesPool, WriteV2Sample{})
			}
			s := &samplesPool[len(samplesPool)-1]
			if err := unmarshalWriteV2Sample(data, s); err != nil {
				return tsPool, samplesPool, histPool, exemplarPool, fmt.Errorf("cannot unmarshal sample: %w", err)
			}
			out.Samples = append(out.Samples, *s)
		case 3: // histograms repeated message
			data, ok := fc.MessageData()
			if !ok {
				return tsPool, samplesPool, histPool, exemplarPool, fmt.Errorf("cannot read histogram data")
			}
			if len(histPool) < cap(histPool) {
				histPool = histPool[:len(histPool)+1]
			} else {
				histPool = append(histPool, WriteV2Histogram{})
			}
			h := &histPool[len(histPool)-1]
			if err := unmarshalWriteV2Histogram(data, h); err != nil {
				return tsPool, samplesPool, histPool, exemplarPool, fmt.Errorf("cannot unmarshal histogram: %w", err)
			}
			out.Histograms = append(out.Histograms, *h)
		case 4: // exemplars repeated message
			data, ok := fc.MessageData()
			if !ok {
				return tsPool, samplesPool, histPool, exemplarPool, fmt.Errorf("cannot read exemplar data")
			}
			if len(exemplarPool) < cap(exemplarPool) {
				exemplarPool = exemplarPool[:len(exemplarPool)+1]
			} else {
				exemplarPool = append(exemplarPool, WriteV2Exemplar{})
			}
			e := &exemplarPool[len(exemplarPool)-1]
			if err := unmarshalWriteV2Exemplar(data, e); err != nil {
				return tsPool, samplesPool, histPool, exemplarPool, fmt.Errorf("cannot unmarshal exemplar: %w", err)
			}
			out.Exemplars = append(out.Exemplars, *e)
		case 5: // metadata message
			data, ok := fc.MessageData()
			if !ok {
				return tsPool, samplesPool, histPool, exemplarPool, fmt.Errorf("cannot read metadata data")
			}
			var mm WriteV2Metadata
			if err := unmarshalWriteV2Metadata(data, &mm); err != nil {
				return tsPool, samplesPool, histPool, exemplarPool, fmt.Errorf("cannot unmarshal metadata: %w", err)
			}
			out.Metadata = &mm
		default:
			// ignore unsupported fields (histograms, exemplars, etc.)
		}
	}
	return tsPool, samplesPool, histPool, exemplarPool, nil
}

func unmarshalWriteV2Sample(src []byte, s *WriteV2Sample) error {
	var fc easyproto.FieldContext
	for len(src) > 0 {
		var err error
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read the next field: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			v, ok := fc.Double()
			if !ok {
				return fmt.Errorf("cannot read sample value")
			}
			s.Value = v
		case 2:
			v, ok := fc.Int64()
			if !ok {
				return fmt.Errorf("cannot read sample timestamp")
			}
			s.Timestamp = v
		case 3: // start_timestamp (optional)
			v, ok := fc.Int64()
			if !ok {
				return fmt.Errorf("cannot read sample start_timestamp")
			}
			s.StartTimestamp = v
		}
	}
	return nil
}

func unmarshalWriteV2Metadata(src []byte, mm *WriteV2Metadata) error {
	var fc easyproto.FieldContext
	for len(src) > 0 {
		var err error
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read the next field: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			v, ok := fc.Uint32()
			if !ok {
				return fmt.Errorf("cannot read metadata type")
			}
			mm.Type = writev2.Metadata_MetricType(v)
		case 3:
			v, ok := fc.Uint32()
			if !ok {
				return fmt.Errorf("cannot read help_ref")
			}
			mm.HelpRef = v
		case 4:
			v, ok := fc.Uint32()
			if !ok {
				return fmt.Errorf("cannot read unit_ref")
			}
			mm.UnitRef = v
		}
	}
	return nil
}

func unmarshalWriteV2Histogram(src []byte, h *WriteV2Histogram) error {
	var fc easyproto.FieldContext
	for len(src) > 0 {
		var err error
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read the next field: %w", err)
		}
		switch fc.FieldNum {
		case 1: // count_int
			v, ok := fc.Uint64()
			if !ok {
				return fmt.Errorf("cannot read count_int")
			}
			h.CountInt = v
			// mark as integer by default (HasCountFloat false)
		case 2: // count_float
			v, ok := fc.Double()
			if !ok {
				return fmt.Errorf("cannot read count_float")
			}
			h.CountFloat = v
			h.HasCountFloat = true
		case 3: // sum
			v, ok := fc.Double()
			if !ok {
				return fmt.Errorf("cannot read sum")
			}
			h.Sum = v
		case 4: // schema
			v, ok := fc.Int32()
			if !ok {
				return fmt.Errorf("cannot read schema")
			}
			h.Schema = v
		case 5: // zero_threshold
			v, ok := fc.Double()
			if !ok {
				return fmt.Errorf("cannot read zero_threshold")
			}
			h.ZeroThreshold = v
		case 6: // zero_count_int
			v, ok := fc.Uint64()
			if !ok {
				return fmt.Errorf("cannot read zero_count_int")
			}
			h.ZeroCountInt = v
		case 7: // zero_count_float
			v, ok := fc.Double()
			if !ok {
				return fmt.Errorf("cannot read zero_count_float")
			}
			h.ZeroCountFloat = v
			h.HasZeroCountFloat = true
		case 8: // negative_spans repeated message
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read negative_span data")
			}
			// unmarshal one BucketSpan
			var bs BucketSpan
			if err := unmarshalBucketSpan(data, &bs); err != nil {
				return fmt.Errorf("cannot unmarshal negative span: %w", err)
			}
			h.NegativeSpans = append(h.NegativeSpans, bs)
		case 9: // negative_deltas packed zigzag64
			vals, ok := fc.UnpackSint64s(make([]int64, 16))
			if ok {
				h.NegativeDeltas = append(h.NegativeDeltas, vals...)
			} else {
				v, ok2 := fc.Int64()
				if !ok2 {
					return fmt.Errorf("cannot read negative_delta")
				}
				h.NegativeDeltas = append(h.NegativeDeltas, v)
			}
		case 10: // negative_counts packed fixed64
			vals, ok := fc.UnpackFixed64s(make([]uint64, 16))
			if ok {
				for _, u := range vals {
					h.NegativeCounts = append(h.NegativeCounts, math.Float64frombits(u))
				}
			} else {
				v, ok2 := fc.Double()
				if !ok2 {
					return fmt.Errorf("cannot read negative_count")
				}
				h.NegativeCounts = append(h.NegativeCounts, v)
			}
		case 11: // positive_spans repeated message
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read positive_span data")
			}
			var bs BucketSpan
			if err := unmarshalBucketSpan(data, &bs); err != nil {
				return fmt.Errorf("cannot unmarshal positive span: %w", err)
			}
			h.PositiveSpans = append(h.PositiveSpans, bs)
		case 12: // positive_deltas packed zigzag64
			vals, ok := fc.UnpackSint64s(make([]int64, 16))
			if ok {
				h.PositiveDeltas = append(h.PositiveDeltas, vals...)
			} else {
				v, ok2 := fc.Int64()
				if !ok2 {
					return fmt.Errorf("cannot read positive_delta")
				}
				h.PositiveDeltas = append(h.PositiveDeltas, v)
			}
		case 13: // positive_counts packed fixed64
			vals, ok := fc.UnpackFixed64s(make([]uint64, 16))
			if ok {
				for _, u := range vals {
					h.PositiveCounts = append(h.PositiveCounts, math.Float64frombits(u))
				}
			} else {
				v, ok2 := fc.Double()
				if !ok2 {
					return fmt.Errorf("cannot read positive_count")
				}
				h.PositiveCounts = append(h.PositiveCounts, v)
			}
		case 14: // reset_hint
			v, ok := fc.Int32()
			if !ok {
				return fmt.Errorf("cannot read reset_hint")
			}
			h.ResetHint = writev2.Histogram_ResetHint(v)
		case 15: // timestamp
			v, ok := fc.Int64()
			if !ok {
				return fmt.Errorf("cannot read histogram timestamp")
			}
			h.Timestamp = v
		case 16: // custom_values packed fixed64
			vals, ok := fc.UnpackFixed64s(make([]uint64, 16))
			if ok {
				for _, u := range vals {
					h.CustomValues = append(h.CustomValues, math.Float64frombits(u))
				}
			} else {
				v, ok2 := fc.Double()
				if !ok2 {
					return fmt.Errorf("cannot read custom_value")
				}
				h.CustomValues = append(h.CustomValues, v)
			}
		case 17: // start_timestamp (optional)
			v, ok := fc.Int64()
			if !ok {
				return fmt.Errorf("cannot read histogram start_timestamp")
			}
			h.StartTimestamp = v
		default:
			// ignore unknown fields
		}
	}
	return nil
}

func unmarshalBucketSpan(src []byte, bs *BucketSpan) error {
	var fc easyproto.FieldContext
	for len(src) > 0 {
		var err error
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read the next field: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			v, ok := fc.Int32()
			if !ok {
				return fmt.Errorf("cannot read offset")
			}
			bs.Offset = v
		case 2:
			v, ok := fc.Uint32()
			if !ok {
				return fmt.Errorf("cannot read length")
			}
			bs.Length = v
		}
	}
	return nil
}

func unmarshalWriteV2Exemplar(src []byte, e *WriteV2Exemplar) error {
	var fc easyproto.FieldContext
	for len(src) > 0 {
		var err error
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read the next field: %w", err)
		}
		switch fc.FieldNum {
		case 1: // labels_refs packed uint32
			vals, ok := fc.UnpackFixed32s(make([]uint32, 8))
			if !ok {
				v, ok2 := fc.Uint32()
				if !ok2 {
					return fmt.Errorf("cannot read exemplar labels_refs")
				}
				e.LabelsRefs = append(e.LabelsRefs, v)
			} else {
				e.LabelsRefs = append(e.LabelsRefs, vals...)
			}
		case 2:
			v, ok := fc.Double()
			if !ok {
				return fmt.Errorf("cannot read exemplar value")
			}
			e.Value = v
		case 3:
			v, ok := fc.Int64()
			if !ok {
				return fmt.Errorf("cannot read exemplar timestamp")
			}
			e.Timestamp = v
		}
	}
	return nil
}

// helper resetters
func ResetWriteV2TimeSeries(s []WriteV2TimeSeries) []WriteV2TimeSeries {
	for i := range s {
		s[i].LabelsRefs = ResetUint32s(s[i].LabelsRefs)
		s[i].Samples = ResetWriteV2Samples(s[i].Samples)
		s[i].Histograms = ResetWriteV2Histograms(s[i].Histograms)
		s[i].Exemplars = ResetWriteV2Exemplars(s[i].Exemplars)
		s[i].Metadata = nil
	}
	return s[:0]
}

func ResetWriteV2Samples(s []WriteV2Sample) []WriteV2Sample {
	for i := range s {
		s[i].Value = 0
		s[i].Timestamp = 0
		s[i].StartTimestamp = 0
	}
	return s[:0]
}

func ResetWriteV2Histograms(s []WriteV2Histogram) []WriteV2Histogram {
	for i := range s {
		s[i].NegativeSpans = s[i].NegativeSpans[:0]
		s[i].NegativeDeltas = s[i].NegativeDeltas[:0]
		s[i].NegativeCounts = s[i].NegativeCounts[:0]
		s[i].PositiveSpans = s[i].PositiveSpans[:0]
		s[i].PositiveDeltas = s[i].PositiveDeltas[:0]
		s[i].PositiveCounts = s[i].PositiveCounts[:0]
		s[i].Timestamp = 0
		s[i].StartTimestamp = 0
		s[i].CustomValues = s[i].CustomValues[:0]
	}
	return s[:0]
}

func ResetWriteV2Exemplars(s []WriteV2Exemplar) []WriteV2Exemplar {
	for i := range s {
		s[i].LabelsRefs = s[i].LabelsRefs[:0]
		s[i].Value = 0
		s[i].Timestamp = 0
	}
	return s[:0]
}

// simple helpers for slices of primitive types used in multiple places
func ResetStrings(s []string) []string { return s[:0] }
func ResetUint32s(s []uint32) []uint32 { return s[:0] }

// small clear utility matching patterns used elsewhere
func clearSlice[T any](s []T) {
	for i := range s {
		var zero T
		s[i] = zero
	}
}

// ---- Helper accessors for WriteV2Histogram -----
// The generated internal representation stores integer and float counts separately.
// These accessor methods provide a compatible API similar to the upstream writev2.Histogram
// so the receiver code can work with the internal type without changes.

// IsFloatHistogram reports whether the histogram uses float64 counts.
func (h *WriteV2Histogram) IsFloatHistogram() bool {
	return h.HasCountFloat
}

// GetZeroCountFloat returns the zero bucket count for float histograms.
func (h *WriteV2Histogram) GetZeroCountFloat() float64 {
	return h.ZeroCountFloat
}

// GetZeroCountInt returns the zero bucket count for integer histograms.
func (h *WriteV2Histogram) GetZeroCountInt() uint64 {
	return h.ZeroCountInt
}

// GetCountFloat returns the overall count for float histograms.
func (h *WriteV2Histogram) GetCountFloat() float64 {
	return h.CountFloat
}

// GetCountInt returns the overall count for integer histograms.
func (h *WriteV2Histogram) GetCountInt() uint64 {
	return h.CountInt
}

// desymbolizeLabels decodes label references into model labels, with given symbols table.
func desymbolizeLabels(b *labels.ScratchBuilder, labelRefs []uint32, symbols []string) (labels.Labels, error) {
	b.Reset()
	for i := 0; i+1 < len(labelRefs); i += 2 {
		nameIdx := int(labelRefs[i])
		valIdx := int(labelRefs[i+1])
		if nameIdx < 0 || nameIdx >= len(symbols) || valIdx < 0 || valIdx >= len(symbols) {
			return labels.Labels{}, fmt.Errorf("invalid symbol reference: nameIdx=%d valIdx=%d symbolsLen=%d", nameIdx, valIdx, len(symbols))
		}
		b.Add(symbols[nameIdx], symbols[valIdx])
	}
	b.Sort()
	return b.Labels(), nil
}
