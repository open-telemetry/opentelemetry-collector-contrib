// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tracebench

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"os"
	"sort"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
)

type Class struct {
	Weight     float64        `json:"weight"`
	Kind       int32          `json:"kind"`
	Scope      string         `json:"scope"`
	Name       string         `json:"name"`
	Attributes map[string]any `json:"attributes"`
	Resource   map[string]any `json:"resource"`
	Extra      int            `json:"extra_attributes"`
	ValueBytes int            `json:"value_bytes"`
	Resources  int            `json:"spans_per_resource"`
}

type Batch struct {
	Weight float64 `json:"weight"`
	Size   int     `json:"size"`
	Class  int     `json:"class"`
	Format string  `json:"format"`
	Gzip   bool    `json:"gzip"`
}

type Stage struct {
	Name   string         `json:"name"`
	Type   string         `json:"type"`
	File   string         `json:"file,omitempty"`
	Config map[string]any `json:"config,omitempty"`
}

type Spec struct {
	Seed          uint64         `json:"seed"`
	Bank          int            `json:"bank"`
	TraceSpans    int            `json:"trace_spans"`
	Classes       []Class        `json:"classes"`
	Batches       []Batch        `json:"batches"`
	Common        []Stage        `json:"common"`
	Sampled       []Stage        `json:"sampled"`
	SamplePercent float64        `json:"sample_percent"`
	Stats         bool           `json:"stats"`
	StatsConfig   map[string]any `json:"stats_config,omitempty"`
}

func DefaultSpec() Spec {
	return Spec{
		Seed: 1, Bank: 8, TraceSpans: 1, SamplePercent: 10, Stats: true,
		Classes: []Class{{
			Weight: 1, Kind: int32(ptrace.SpanKindClient), Scope: "example.client",
			Name: "GET /example", Extra: 8, ValueBytes: 16, Resources: 16,
			Attributes: map[string]any{"old.field": "example", "http.request.method": "GET"},
			Resource:   map[string]any{"service.name": "example", "deployment.environment": "synthetic"},
		}},
		Batches: []Batch{{Weight: 1, Size: 256, Class: -1, Format: "proto"}},
		Common: []Stage{{Name: "rewrite", Type: "transform", Config: map[string]any{
			"error_mode": "propagate",
			"trace_statements": []any{map[string]any{
				"context": "span", "statements": []any{`replace_all_patterns(attributes, "key", "old\\.field", "new.field")`},
			}},
		}}},
	}
}

func ReadSpec(path string) (Spec, error) {
	if path == "" {
		s := DefaultSpec()
		return s, s.Validate()
	}
	file, err := os.Open(path)
	if err != nil {
		return Spec{}, err
	}
	defer file.Close()
	s := Spec{TraceSpans: 1}
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&s); err != nil {
		return Spec{}, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return Spec{}, errors.New("expected exactly one workload document")
	}
	return s, s.Validate()
}

func positiveWeight(value float64) bool {
	return value > 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

func (s Spec) Validate() error {
	if s.Bank < 1 || s.Bank > 4096 || len(s.Classes) == 0 || len(s.Batches) == 0 {
		return errors.New("bank must be 1..4096 and classes/batches must be nonempty")
	}
	if s.TraceSpans < 1 || s.TraceSpans > 65536 {
		return errors.New("trace_spans must be 1..65536")
	}
	if math.IsNaN(s.SamplePercent) || s.SamplePercent < 0 || s.SamplePercent > 100 {
		return errors.New("sample_percent must be finite and in 0..100")
	}
	var classWeight float64
	for _, c := range s.Classes {
		classWeight += c.Weight
		if c.Weight < 0 || math.IsNaN(c.Weight) || math.IsInf(c.Weight, 0) || c.Kind < 0 || c.Kind > 5 || c.Extra < 0 || c.Extra > 1024 ||
			c.ValueBytes < 0 || c.ValueBytes > 65536 || c.Resources < 1 {
			return errors.New("invalid class weight, kind, attribute count, value size, or resource grouping")
		}
	}
	if !positiveWeight(classWeight) {
		return errors.New("classes need a finite positive total weight")
	}
	for _, b := range s.Batches {
		if !positiveWeight(b.Weight) || b.Size < 1 || b.Size > 65536 || b.Class < -1 || b.Class >= len(s.Classes) ||
			(b.Format != "proto" && b.Format != "json") {
			return errors.New("invalid batch weight, size, class, or format")
		}
	}
	for _, group := range [][]Stage{s.Common, s.Sampled} {
		for _, stage := range group {
			if stage.Name == "" || stage.Type == "" || (stage.File == "") == (stage.Config == nil) {
				return errors.New("stages need name/type and exactly one of file or config")
			}
		}
	}
	return nil
}

func pick(random *rand.Rand, weights []float64) int {
	return weightedIndex(weights, random.Float64())
}

func weightedIndex(weights []float64, quantile float64) int {
	var total float64
	for _, weight := range weights {
		total += weight
	}
	point := quantile * total
	for i, weight := range weights {
		point -= weight
		if point < 0 {
			return i
		}
	}
	return len(weights) - 1
}

func putAttributes(target pcommon.Map, attributes map[string]any) error {
	keys := make([]string, 0, len(attributes))
	for key := range attributes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if err := target.PutEmpty(key).FromRaw(attributes[key]); err != nil {
			return fmt.Errorf("attribute %q: %w", key, err)
		}
	}
	return nil
}

func makeTraces(s Spec, batch Batch, random *rand.Rand, now time.Time) (ptrace.Traces, error) {
	data := ptrace.NewTraces()
	weights := make([]float64, len(s.Classes))
	for i, class := range s.Classes {
		weights[i] = class.Weight
	}
	groups := make([]ptrace.SpanSlice, len(s.Classes))
	counts := make([]int, len(s.Classes))
	var traceID pcommon.TraceID
	var parentID pcommon.SpanID
	for index := range batch.Size {
		classIndex := batch.Class
		if classIndex < 0 {
			classIndex = pick(random, weights)
		}
		class := s.Classes[classIndex]
		if counts[classIndex] == 0 || counts[classIndex] >= class.Resources {
			resource := data.ResourceSpans().AppendEmpty()
			if err := putAttributes(resource.Resource().Attributes(), class.Resource); err != nil {
				return ptrace.Traces{}, err
			}
			scope := resource.ScopeSpans().AppendEmpty()
			scope.Scope().SetName(class.Scope)
			groups[classIndex] = scope.Spans()
			counts[classIndex] = 0
		}
		counts[classIndex]++
		span := groups[classIndex].AppendEmpty()
		if index%s.TraceSpans == 0 {
			binary.LittleEndian.PutUint64(traceID[:8], random.Uint64())
			binary.LittleEndian.PutUint64(traceID[8:], random.Uint64())
			parentID = pcommon.SpanID{}
		}
		var spanID pcommon.SpanID
		binary.LittleEndian.PutUint64(spanID[:], random.Uint64())
		span.SetTraceID(traceID)
		span.SetSpanID(spanID)
		span.SetParentSpanID(parentID)
		if index%s.TraceSpans == 0 {
			parentID = spanID
		}
		span.SetKind(ptrace.SpanKind(class.Kind))
		span.SetName(class.Name)
		span.SetStartTimestamp(pcommon.NewTimestampFromTime(now.Add(-time.Millisecond)))
		span.SetEndTimestamp(pcommon.NewTimestampFromTime(now))
		if err := putAttributes(span.Attributes(), class.Attributes); err != nil {
			return ptrace.Traces{}, err
		}
		for j := range class.Extra {
			var value strings.Builder
			for value.Len() < class.ValueBytes {
				fmt.Fprintf(&value, "%016x", random.Uint64())
			}
			span.Attributes().PutStr(fmt.Sprintf("synthetic.attribute.%d", j), value.String()[:class.ValueBytes])
		}
	}
	return data, nil
}

type Request struct {
	Wire   []byte
	Format string
	Gzip   bool
	Spans  int
}

func MakeBank(s Spec) ([]Request, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	random := rand.New(rand.NewPCG(s.Seed, s.Seed^0x9e3779b97f4a7c15))
	weights := make([]float64, len(s.Batches))
	for i, batch := range s.Batches {
		weights[i] = batch.Weight
	}
	requests := make([]Request, s.Bank)
	now := time.Now()
	for i := range requests {
		batch := s.Batches[weightedIndex(weights, (float64(i)+0.5)/float64(s.Bank))]
		data, err := makeTraces(s, batch, random, now)
		if err != nil {
			return nil, err
		}
		request := ptraceotlp.NewExportRequestFromTraces(data)
		var wire []byte
		if batch.Format == "json" {
			wire, err = request.MarshalJSON()
		} else {
			wire, err = request.MarshalProto()
		}
		if err != nil {
			return nil, err
		}
		if batch.Gzip {
			var buffer bytes.Buffer
			writer := gzip.NewWriter(&buffer)
			if _, err := writer.Write(wire); err != nil {
				return nil, err
			}
			if err := writer.Close(); err != nil {
				return nil, err
			}
			wire = buffer.Bytes()
		}
		requests[i] = Request{Wire: wire, Format: batch.Format, Gzip: batch.Gzip, Spans: batch.Size}
	}
	random.Shuffle(len(requests), func(i, j int) { requests[i], requests[j] = requests[j], requests[i] })
	return requests, nil
}

func (r Request) Decode() (ptrace.Traces, error) {
	wire := r.Wire
	if r.Gzip {
		reader, err := gzip.NewReader(bytes.NewReader(wire))
		if err != nil {
			return ptrace.Traces{}, err
		}
		wire, err = io.ReadAll(reader)
		closeErr := reader.Close()
		if err != nil {
			return ptrace.Traces{}, err
		}
		if closeErr != nil {
			return ptrace.Traces{}, closeErr
		}
	}
	request := ptraceotlp.NewExportRequest()
	var err error
	if r.Format == "json" {
		err = request.UnmarshalJSON(wire)
	} else {
		err = request.UnmarshalProto(wire)
	}
	return request.Traces(), err
}
