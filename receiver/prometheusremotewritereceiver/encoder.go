// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewritereceiver // import "go.opentelemetry.io/collector/receiver/otlpreceiver"

import (
	"bytes"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/gogo/protobuf/proto"
	spb "google.golang.org/genproto/googleapis/rpc/status"

	p "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusremotewritereceiver/proto"
)

const (
	pbContentType   = "application/x-protobuf"
	jsonContentType = "application/json"
)

var (
	pbEncoder       = &protoEncoder{}
	jsEncoder       = &jsonEncoder{}
	jsonMarshaler   = &jsonpb.Marshaler{}
	jsonUnmarshaler = &jsonpb.Unmarshaler{}
)

type encoder interface {
	unmarshalWriteRequest(buf []byte) (p.WriteRequest, error)

	//marshalMetricsResponse(p.Wr) ([]byte, error)

	marshalStatus(rsp *spb.Status) ([]byte, error)

	contentType() string
}

type protoEncoder struct{}

func (protoEncoder) unmarshalWriteRequest(buf []byte) (p.WriteRequest, error) {
	req := p.WriteRequest{}
	err := req.Unmarshal(buf)
	return req, err
}

func (protoEncoder) marshalStatus(resp *spb.Status) ([]byte, error) {
	return proto.Marshal(resp)
}

func (protoEncoder) contentType() string {
	return pbContentType
}

type jsonEncoder struct{}

func (jsonEncoder) unmarshalWriteRequest(buf []byte) (p.WriteRequest, error) {
	req := p.WriteRequest{}
	//err := req.Unmarshal(buf)

	err := jsonUnmarshaler.Unmarshal(bytes.NewReader(buf), &req)
	return req, err
}

func (jsonEncoder) marshalStatus(resp *spb.Status) ([]byte, error) {
	buf := new(bytes.Buffer)
	err := jsonMarshaler.Marshal(buf, resp)
	return buf.Bytes(), err
}

func (jsonEncoder) contentType() string {
	return jsonContentType
}
