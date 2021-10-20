// Copyright 2019, OpenTelemetry Authors
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

package jaegerthrifthttpexporter

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/jaegertracing/jaeger/model"
	jaegerThriftConverter "github.com/jaegertracing/jaeger/model/converter/thrift/jaeger"
	"github.com/jaegertracing/jaeger/thrift-gen/jaeger"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/model/pdata"

	jaegertranslator "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger"
)

// Default timeout for http request in seconds
const defaultHTTPTimeout = time.Second * 5

// newTracesExporter returns a new Jaeger Thrift over HTTP exporter.
// The exporterName is the name to be used in the observability of the exporter.
// The httpAddress should be the URL of the collector to handle POST requests,
// typically something like: http://hostname:14268/api/traces.
// The headers parameter is used to add entries to the POST message set to the
// collector.
// The timeout is used to set the timeout for the HTTP requests, if the
// value is equal or smaller than zero the default of 5 seconds is used.
func newTracesExporter(
	config *Config,
	params component.ExporterCreateSettings,
) (component.TracesExporter, error) {

	clientTimeout := defaultHTTPTimeout
	if config.Timeout != 0 {
		clientTimeout = config.Timeout
	}
	s := &jaegerThriftHTTPSender{
		url:     config.URL,
		headers: config.Headers,
		client:  &http.Client{Timeout: clientTimeout},
	}

	return exporterhelper.NewTracesExporter(
		config,
		params,
		s.pushTraceData)
}

// jaegerThriftHTTPSender forwards spans encoded in the jaeger thrift
// format to a http server.
type jaegerThriftHTTPSender struct {
	url     string
	headers map[string]string
	client  *http.Client
}

func (s *jaegerThriftHTTPSender) pushTraceData(
	ctx context.Context,
	td pdata.Traces,
) error {
	batches, err := jaegertranslator.InternalTracesToJaegerProto(td)
	if err != nil {
		return consumererror.NewPermanent(fmt.Errorf("failed to push trace data via Jaeger Thrift HTTP exporter: %w", err))
	}

	for i := 0; i < len(batches); i++ {
		body, err := serializeThrift(ctx, batches[i])
		if err != nil {
			return err
		}

		req, err := http.NewRequest("POST", s.url, body)
		if err != nil {
			return err
		}

		req.Header.Set("Content-Type", "application/x-thrift")
		if s.headers != nil {
			for k, v := range s.headers {
				req.Header.Set(k, v)
			}
		}

		resp, err := s.client.Do(req)
		if err != nil {
			return err
		}

		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()

		if resp.StatusCode >= http.StatusBadRequest {
			err = fmt.Errorf(
				"HTTP %d %q",
				resp.StatusCode,
				http.StatusText(resp.StatusCode))
			return err
		}
	}

	return nil
}

func serializeThrift(ctx context.Context, batch *model.Batch) (*bytes.Buffer, error) {
	thriftSpans := jaegerThriftConverter.FromDomain(batch.GetSpans())
	thriftProcess := jaeger.Process{
		ServiceName: batch.GetProcess().GetServiceName(),
		Tags:        convertTagsToThrift(batch.GetProcess().GetTags()),
	}
	thriftBatch := jaeger.Batch{
		Spans:   thriftSpans,
		Process: &thriftProcess,
	}
	t := thrift.NewTMemoryBuffer()
	p := thrift.NewTBinaryProtocolConf(t, nil)
	if err := thriftBatch.Write(ctx, p); err != nil {
		return nil, err
	}
	return t.Buffer, nil
}

func convertTagsToThrift(tags []model.KeyValue) []*jaeger.Tag {
	thriftTags := make([]*jaeger.Tag, 0, len(tags))

	for i := 0; i < len(tags); i++ {
		tag := tags[i]
		thriftTag := &jaeger.Tag{Key: tag.GetKey()}
		switch tag.GetVType() {
		case model.ValueType_STRING:
			str := tag.GetVStr()
			thriftTag.VStr = &str
			thriftTag.VType = jaeger.TagType_STRING
		case model.ValueType_INT64:
			i := tag.GetVInt64()
			thriftTag.VLong = &i
			thriftTag.VType = jaeger.TagType_LONG
		case model.ValueType_BOOL:
			b := tag.GetVBool()
			thriftTag.VBool = &b
			thriftTag.VType = jaeger.TagType_BOOL
		case model.ValueType_FLOAT64:
			d := tag.GetVFloat64()
			thriftTag.VDouble = &d
			thriftTag.VType = jaeger.TagType_DOUBLE
		default:
			str := "<Unknown tag type for key \"" + tag.GetKey() + "\">"
			thriftTag.VStr = &str
			thriftTag.VType = jaeger.TagType_STRING
		}
		thriftTags = append(thriftTags, thriftTag)
	}

	return thriftTags
}
