// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package datadogreceiver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/DataDog/datadog-agent/pkg/trace/exportable/pb"
	"github.com/gorilla/mux"
	"github.com/tinylib/msgp/msgp"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/obsreport"
	tracetranslator "go.opentelemetry.io/collector/translator/trace"
	"io"
	"math"
	"mime"
	"net/http"
	"strings"
	"unicode/utf8"

	"sync"
)

type datadogReceiver struct {
	config       *Config
	params       component.ReceiverCreateParams
	nextConsumer consumer.Traces
	server       *http.Server
	longLivedCtx context.Context

	mu        sync.Mutex
	startOnce sync.Once
	stopOnce  sync.Once
}

func newDataDogReceiver(config *Config, nextConsumer consumer.Traces, params component.ReceiverCreateParams) (component.TracesReceiver, error) {
	if nextConsumer == nil {
		return nil, componenterror.ErrNilNextConsumer
	}
	return &datadogReceiver{
		params:       params,
		config:       config,
		nextConsumer: nextConsumer,
	}, nil
}

const collectorHTTPTransport = "http_collector"

var bufferPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

func getBuffer() *bytes.Buffer {
	buffer := bufferPool.Get().(*bytes.Buffer)
	buffer.Reset()
	return buffer
}

func putBuffer(buffer *bytes.Buffer) {
	bufferPool.Put(buffer)
}

func (ddr *datadogReceiver) Start(ctx context.Context, host component.Host) error {
	var err error
	ddr.longLivedCtx = obsreport.ReceiverContext(ctx, "datadog", "http")
	ddr.startOnce.Do(func() {
		ddmux := mux.NewRouter()
		ddmux.HandleFunc("/v0.3/traces", ddr.handleTraces)
		ddmux.HandleFunc("/v0.4/traces", ddr.handleTraces)
		ddmux.HandleFunc("/v0.5/traces", ddr.handleTraces05)
		ddr.server = &http.Server{
			Handler: ddmux,
			Addr:    ddr.config.HTTPServerSettings.Endpoint,
		}
		if err := ddr.server.ListenAndServe(); err != http.ErrServerClosed {
			host.ReportFatalError(fmt.Errorf("error starting datadog receiver: %v", err))
		}
	})
	return err
}

func (ddr *datadogReceiver) Shutdown(ctx context.Context) error {
	ddr.stopOnce.Do(func() {
		_ = ddr.server.Shutdown(context.Background())
	})
	return nil
}

func (ddr *datadogReceiver) handleTraces(w http.ResponseWriter, req *http.Request) {
	obsreport.StartTraceDataReceiveOp(ddr.longLivedCtx, "datadogReceiver", "http")
	var traces pb.Traces
	err := decodeRequest(req, &traces)
	if err != nil {
		http.Error(w, "Unable to unmarshal reqs", http.StatusInternalServerError)
		obsreport.EndTraceDataReceiveOp(ddr.longLivedCtx, typeStr, 0, err)
		return
	}

	ddr.processTraces(req.Context(), traces, w)
}

func (ddr *datadogReceiver) handleTraces05(w http.ResponseWriter, req *http.Request) {
	obsreport.StartTraceDataReceiveOp(ddr.longLivedCtx, typeStr, collectorHTTPTransport)
	var traces pb.Traces
	buf := getBuffer()
	if _, err := io.Copy(buf, req.Body); err != nil {
		http.Error(w, "Unable to unmarshal reqs", http.StatusInternalServerError)
		obsreport.EndTraceDataReceiveOp(ddr.longLivedCtx, typeStr, 0, err)
		return
	}
	err := ddr.UnmarshalMsgDictionary(&traces, buf.Bytes())
	if err != nil {
		http.Error(w, "Unable to unmarshal reqs", http.StatusInternalServerError)
		obsreport.EndTraceDataReceiveOp(ddr.longLivedCtx, typeStr, 0, err)
		return
	}
	putBuffer(buf)
	ddr.processTraces(req.Context(), traces, w)
}

func (ddr *datadogReceiver) processTraces(ctx context.Context, traces pb.Traces, w http.ResponseWriter) {
	newTraces := pdata.NewTraces()
	newTraces.ResourceSpans().Resize(len(traces))
	totalSpansCount := 0
	for i, trace := range traces {
		totalSpansCount += len(trace)
		newTraces.ResourceSpans().At(i).InstrumentationLibrarySpans().Resize(len(trace))
		for i2, span := range trace {
			newSpan := pdata.NewSpan()
			newSpan.SetTraceID(tracetranslator.UInt64ToTraceID(span.TraceID, span.TraceID))
			newSpan.SetSpanID(tracetranslator.UInt64ToSpanID(span.SpanID))
			newSpan.SetStartTimestamp(pdata.Timestamp(span.Start))
			newSpan.SetEndTimestamp(pdata.Timestamp(span.Start + span.Duration))
			newSpan.SetParentSpanID(tracetranslator.UInt64ToSpanID(span.ParentID))
			newSpan.SetName(span.Name)
			for k, v := range span.GetMeta() {
				newSpan.Attributes().InsertString(k, v)
			}
			if span.Type == "web" {
				newSpan.SetKind(pdata.SpanKindSERVER)
			} else if span.Type == "client" {
				newSpan.SetKind(pdata.SpanKindCLIENT)
			} else {
				newSpan.SetKind(pdata.SpanKindINTERNAL)
			}
			newTraces.ResourceSpans().At(i).InstrumentationLibrarySpans().At(i2).Spans().Append(newSpan)
		}
	}
	err := ddr.nextConsumer.ConsumeTraces(ctx, newTraces)
	if err != nil {
		http.Error(w, "Trace consumer errored out", http.StatusInternalServerError)
		obsreport.EndTraceDataReceiveOp(ddr.longLivedCtx, typeStr, totalSpansCount, err)
	}
	obsreport.EndTraceDataReceiveOp(ddr.longLivedCtx, typeStr, totalSpansCount, nil)
	_, _ = w.Write([]byte("OK"))
}

/// Thanks Datadog!
func decodeRequest(req *http.Request, dest *pb.Traces) error {
	switch mediaType := getMediaType(req); mediaType {
	case "application/msgpack":
		return msgp.Decode(req.Body, dest)
	case "application/json":
		fallthrough
	case "text/json":
		fallthrough
	case "":
		return json.NewDecoder(req.Body).Decode(dest)
	default:
		// do our best
		if err1 := json.NewDecoder(req.Body).Decode(dest); err1 != nil {
			if err2 := msgp.Decode(req.Body, dest); err2 != nil {
				return fmt.Errorf("could not decode JSON (%q), nor Msgpack (%q)", err1, err2)
			}
		}
		return nil
	}
}

func getMediaType(req *http.Request) string {
	mt, _, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
	if err != nil {
		return "application/json"
	}
	return mt
}

// dictionaryString reads an int from decoder dc and returns the string
// at that index from dict.
func dictionaryString(bts []byte, dict []string) (string, []byte, error) {
	var (
		ui  uint32
		err error
	)
	ui, bts, err = msgp.ReadUint32Bytes(bts)
	if err != nil {
		return "", bts, err
	}
	idx := int(ui)
	if idx >= len(dict) {
		return "", bts, fmt.Errorf("dictionary index %d out of range", idx)
	}
	return dict[idx], bts, nil
}

// UnmarshalMsgDictionary decodes a trace using the specification from the v0.5 endpoint.
// For details, see the documentation for endpoint v0.5 in pkg/trace/api/version.go
func (ddr *datadogReceiver) UnmarshalMsgDictionary(t *pb.Traces, bts []byte) error {
	var err error
	if _, bts, err = msgp.ReadArrayHeaderBytes(bts); err != nil {
		return err
	}
	// read dictionary
	var sz uint32
	if sz, bts, err = msgp.ReadArrayHeaderBytes(bts); err != nil {
		return err
	}
	dict := make([]string, sz)
	for i := range dict {
		var str string
		str, bts, err = parseStringBytes(bts)
		if err != nil {
			return err
		}
		dict[i] = str
	}
	// read traces
	sz, bts, err = msgp.ReadArrayHeaderBytes(bts)
	if err != nil {
		return err
	}
	if cap(*t) >= int(sz) {
		*t = (*t)[:sz]
	} else {
		*t = make(pb.Traces, sz)
	}
	for i := range *t {
		sz, bts, err = msgp.ReadArrayHeaderBytes(bts)
		if err != nil {
			return err
		}
		if cap((*t)[i]) >= int(sz) {
			(*t)[i] = (*t)[i][:sz]
		} else {
			(*t)[i] = make(pb.Trace, sz)
		}
		for j := range (*t)[i] {
			if (*t)[i][j] == nil {
				(*t)[i][j] = new(pb.Span)
			}
			if bts, err = ddr.SpanUnmarshalMsgDictionary((*t)[i][j], bts, dict); err != nil {
				return err
			}
		}
	}
	return nil
}

// spanPropertyCount specifies the number of top-level properties that a span
// has.
const spanPropertyCount = 12

// UnmarshalMsgDictionary decodes a span from the given decoder dc, looking up strings
// in the given dictionary dict. For details, see the documentation for endpoint v0.5
// in pkg/trace/api/version.go
func (ddr *datadogReceiver) SpanUnmarshalMsgDictionary(z *pb.Span, bts []byte, dict []string) ([]byte, error) {
	var (
		sz  uint32
		err error
	)
	sz, bts, err = msgp.ReadArrayHeaderBytes(bts)
	if err != nil {
		return bts, err
	}
	if sz != spanPropertyCount {
		return bts, errors.New("encoded span needs exactly 12 elements in array")
	}
	// Service (0)
	z.Service, bts, err = dictionaryString(bts, dict)
	if err != nil {
		return bts, err
	}
	// Name (1)
	z.Name, bts, err = dictionaryString(bts, dict)
	if err != nil {
		return bts, err
	}
	// Resource (2)
	z.Resource, bts, err = dictionaryString(bts, dict)
	if err != nil {
		return bts, err
	}
	// TraceID (3)
	z.TraceID, bts, err = parseUint64Bytes(bts)
	if err != nil {
		return bts, err
	}
	// SpanID (4)
	z.SpanID, bts, err = parseUint64Bytes(bts)
	if err != nil {
		return bts, err
	}
	// ParentID (5)
	z.ParentID, bts, err = parseUint64Bytes(bts)
	if err != nil {
		return bts, err
	}
	// Start (6)
	z.Start, bts, err = parseInt64Bytes(bts)
	if err != nil {
		return bts, err
	}
	// Duration (7)
	z.Duration, bts, err = parseInt64Bytes(bts)
	if err != nil {
		return bts, err
	}
	// Error (8)
	z.Error, bts, err = parseInt32Bytes(bts)
	if err != nil {
		return bts, err
	}
	// Meta (9)
	sz, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return bts, err
	}
	if z.Meta == nil && sz > 0 {
		z.Meta = make(map[string]string, sz)
	} else if len(z.Meta) > 0 {
		for key := range z.Meta {
			delete(z.Meta, key)
		}
	}
	for sz > 0 {
		sz--
		var key, val string
		key, bts, err = dictionaryString(bts, dict)
		if err != nil {
			return bts, err
		}
		val, bts, err = dictionaryString(bts, dict)
		if err != nil {
			return bts, err
		}
		z.Meta[key] = val
	}
	// Metrics (10)
	sz, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return bts, err
	}
	if z.Metrics == nil && sz > 0 {
		z.Metrics = make(map[string]float64, sz)
	} else if len(z.Metrics) > 0 {
		for key := range z.Metrics {
			delete(z.Metrics, key)
		}
	}
	for sz > 0 {
		sz--
		var (
			key string
			val float64
		)
		key, bts, err = dictionaryString(bts, dict)
		if err != nil {
			return bts, err
		}
		val, bts, err = parseFloat64Bytes(bts)
		if err != nil {
			return bts, err
		}
		z.Metrics[key] = val
	}
	// Type (11)
	z.Type, bts, err = dictionaryString(bts, dict)
	if err != nil {
		return bts, err
	}
	return bts, nil
}

// repairUTF8 ensures all characters in s are UTF-8 by replacing non-UTF-8 characters
// with the replacement char �
func repairUTF8(s string) string {
	in := strings.NewReader(s)
	var out bytes.Buffer
	out.Grow(len(s))

	for {
		r, _, err := in.ReadRune()
		if err != nil {
			// note: by contract, if `in` contains non-valid utf-8, no error is returned. Rather the utf-8 replacement
			// character is returned. Therefore, the only error should usually be io.EOF indicating end of string.
			// If any other error is returned by chance, we quit as well, outputting whatever part of the string we
			// had already constructed.
			return out.String()
		}
		out.WriteRune(r)
	}
}

// parseStringBytes reads the next type in the msgpack payload and
// converts the BinType or the StrType in a valid string.
func parseStringBytes(bts []byte) (string, []byte, error) {
	// read the generic representation type without decoding
	t := msgp.NextType(bts)

	var (
		err error
		i   []byte
	)
	switch t {
	case msgp.BinType:
		i, bts, err = msgp.ReadBytesZC(bts)
	case msgp.StrType:
		i, bts, err = msgp.ReadStringZC(bts)
	default:
		return "", bts, msgp.TypeError{Encoded: t, Method: msgp.StrType}
	}
	if err != nil {
		return "", bts, err
	}
	if utf8.Valid(i) {
		return string(i), bts, nil
	}
	return repairUTF8(msgp.UnsafeString(i)), bts, nil
}

// cast to int32 values that are int32 but that are sent in uint32
// over the wire. Set to 0 if they overflow the MaxInt32 size. This
// cast should be used ONLY while decoding int32 values that are
// sent as uint32 to reduce the payload size, otherwise the approach
// is not correct in the general sense.
func castInt32(v uint32) (int32, bool) {
	if v > math.MaxInt32 {
		return 0, false
	}
	return int32(v), true
}

// parseInt32Bytes parses an int32 even if the sent value is an uint32;
// this is required because the encoding library could remove bytes from the encoded
// payload to reduce the size, if they're not needed.
func parseInt32Bytes(bts []byte) (int32, []byte, error) {
	// read the generic representation type without decoding
	t := msgp.NextType(bts)

	var (
		i   int32
		u   uint32
		err error
	)
	switch t {
	case msgp.IntType:
		i, bts, err = msgp.ReadInt32Bytes(bts)
		if err != nil {
			return 0, bts, err
		}
		return i, bts, nil
	case msgp.UintType:
		u, bts, err = msgp.ReadUint32Bytes(bts)
		if err != nil {
			return 0, bts, err
		}

		// force-cast
		i, ok := castInt32(u)
		if !ok {
			return 0, bts, errors.New("found uint32, overflows int32")
		}
		return i, bts, nil
	default:
		return 0, bts, msgp.TypeError{Encoded: t, Method: msgp.IntType}
	}
}

func parseFloat64Bytes(bts []byte) (float64, []byte, error) {
	// read the generic representation type without decoding
	t := msgp.NextType(bts)

	var err error
	switch t {
	case msgp.IntType:
		var i int64
		i, bts, err = msgp.ReadInt64Bytes(bts)
		if err != nil {
			return 0, bts, err
		}

		return float64(i), bts, nil
	case msgp.UintType:
		var i uint64
		i, bts, err = msgp.ReadUint64Bytes(bts)
		if err != nil {
			return 0, bts, err
		}

		return float64(i), bts, nil
	case msgp.Float64Type:
		var f float64
		f, bts, err = msgp.ReadFloat64Bytes(bts)
		if err != nil {
			return 0, bts, err
		}

		return f, bts, nil
	default:
		return 0, bts, msgp.TypeError{Encoded: t, Method: msgp.Float64Type}
	}
}

// cast to int64 values that are int64 but that are sent in uint64
// over the wire. Set to 0 if they overflow the MaxInt64 size. This
// cast should be used ONLY while decoding int64 values that are
// sent as uint64 to reduce the payload size, otherwise the approach
// is not correct in the general sense.
func castInt64(v uint64) (int64, bool) {
	if v > math.MaxInt64 {
		return 0, false
	}
	return int64(v), true
}

// parseInt64Bytes parses an int64 even if the sent value is an uint64;
// this is required because the encoding library could remove bytes from the encoded
// payload to reduce the size, if they're not needed.
func parseInt64Bytes(bts []byte) (int64, []byte, error) {
	// read the generic representation type without decoding
	t := msgp.NextType(bts)

	var (
		i   int64
		u   uint64
		err error
	)
	switch t {
	case msgp.IntType:
		i, bts, err = msgp.ReadInt64Bytes(bts)
		if err != nil {
			return 0, bts, err
		}
		return i, bts, nil
	case msgp.UintType:
		u, bts, err = msgp.ReadUint64Bytes(bts)
		if err != nil {
			return 0, bts, err
		}

		// force-cast
		i, ok := castInt64(u)
		if !ok {
			return 0, bts, errors.New("found uint64, overflows int64")
		}
		return i, bts, nil
	default:
		return 0, bts, msgp.TypeError{Encoded: t, Method: msgp.IntType}
	}
}

func parseUint64Bytes(bts []byte) (uint64, []byte, error) {
	// read the generic representation type without decoding
	t := msgp.NextType(bts)

	var (
		i   int64
		u   uint64
		err error
	)
	switch t {
	case msgp.UintType:
		u, bts, err = msgp.ReadUint64Bytes(bts)
		if err != nil {
			return 0, bts, err
		}
		return u, bts, err
	case msgp.IntType:
		i, bts, err = msgp.ReadInt64Bytes(bts)
		if err != nil {
			return 0, bts, err
		}
		return uint64(i), bts, nil
	default:
		return 0, bts, msgp.TypeError{Encoded: t, Method: msgp.IntType}
	}
}
