// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package encoder // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/libhoneyreceiver/encoder"

import (
	"bytes"

	"github.com/gogo/protobuf/jsonpb"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	spb "google.golang.org/genproto/googleapis/rpc/status"
)

const (
	PbContentType      = "application/x-protobuf"
	JSONContentType    = "application/json"
	MsgpackContentType = "application/msgpack"
)

var (
	JsEncoder       = &JSONEncoder{}
	JSONPbMarshaler = &jsonpb.Marshaler{}
	MpEncoder       = &msgpackEncoder{}
)

type Encoder interface {
	UnmarshalTracesRequest(buf []byte) (ptraceotlp.ExportRequest, error)
	UnmarshalMetricsRequest(buf []byte) (pmetricotlp.ExportRequest, error)
	UnmarshalLogsRequest(buf []byte) (plogotlp.ExportRequest, error)

	MarshalTracesResponse(ptraceotlp.ExportResponse) ([]byte, error)
	MarshalMetricsResponse(pmetricotlp.ExportResponse) ([]byte, error)
	MarshalLogsResponse(plogotlp.ExportResponse) ([]byte, error)

	MarshalStatus(rsp *spb.Status) ([]byte, error)

	ContentType() string
}

type JSONEncoder struct{}

func (JSONEncoder) UnmarshalTracesRequest(buf []byte) (ptraceotlp.ExportRequest, error) {
	req := ptraceotlp.NewExportRequest()
	err := req.UnmarshalJSON(buf)
	return req, err
}

func (JSONEncoder) UnmarshalMetricsRequest(buf []byte) (pmetricotlp.ExportRequest, error) {
	req := pmetricotlp.NewExportRequest()
	err := req.UnmarshalJSON(buf)
	return req, err
}

func (JSONEncoder) UnmarshalLogsRequest(buf []byte) (plogotlp.ExportRequest, error) {
	req := plogotlp.NewExportRequest()
	err := req.UnmarshalJSON(buf)
	return req, err
}

func (JSONEncoder) MarshalTracesResponse(resp ptraceotlp.ExportResponse) ([]byte, error) {
	return resp.MarshalJSON()
}

func (JSONEncoder) MarshalMetricsResponse(resp pmetricotlp.ExportResponse) ([]byte, error) {
	return resp.MarshalJSON()
}

func (JSONEncoder) MarshalLogsResponse(resp plogotlp.ExportResponse) ([]byte, error) {
	return resp.MarshalJSON()
}

func (JSONEncoder) MarshalStatus(resp *spb.Status) ([]byte, error) {
	buf := new(bytes.Buffer)
	err := JSONPbMarshaler.Marshal(buf, resp)
	return buf.Bytes(), err
}

func (JSONEncoder) ContentType() string {
	return JSONContentType
}

// messagepack responses seem to work in JSON so leaving this alone for now.
type msgpackEncoder struct{}

func (msgpackEncoder) UnmarshalTracesRequest(buf []byte) (ptraceotlp.ExportRequest, error) {
	req := ptraceotlp.NewExportRequest()
	err := req.UnmarshalJSON(buf)
	return req, err
}

func (msgpackEncoder) UnmarshalMetricsRequest(buf []byte) (pmetricotlp.ExportRequest, error) {
	req := pmetricotlp.NewExportRequest()
	err := req.UnmarshalJSON(buf)
	return req, err
}

func (msgpackEncoder) UnmarshalLogsRequest(buf []byte) (plogotlp.ExportRequest, error) {
	req := plogotlp.NewExportRequest()
	err := req.UnmarshalJSON(buf)
	return req, err
}

func (msgpackEncoder) MarshalTracesResponse(resp ptraceotlp.ExportResponse) ([]byte, error) {
	return resp.MarshalJSON()
}

func (msgpackEncoder) MarshalMetricsResponse(resp pmetricotlp.ExportResponse) ([]byte, error) {
	return resp.MarshalJSON()
}

func (msgpackEncoder) MarshalLogsResponse(resp plogotlp.ExportResponse) ([]byte, error) {
	return resp.MarshalJSON()
}

func (msgpackEncoder) MarshalStatus(resp *spb.Status) ([]byte, error) {
	buf := new(bytes.Buffer)
	err := JSONPbMarshaler.Marshal(buf, resp)
	return buf.Bytes(), err
}

func (msgpackEncoder) ContentType() string {
	return MsgpackContentType
}
