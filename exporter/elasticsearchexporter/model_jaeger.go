// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"encoding/json"
	"github.com/jaegertracing/jaeger/plugin/storage/es/spanstore/dbmodel"
	otel_jaeger "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type encodeJaegerModel struct {
}

func (m *encodeJaegerModel) encodeSpan(resource pcommon.Resource, scope pcommon.InstrumentationScope, span ptrace.Span) ([]byte, error) {
	//TODO: make this domain converter configurable, refs: https://www.jaegertracing.io/docs/1.46/cli/#jaeger-all-in-one-elasticsearch
	fromDomain := dbmodel.NewFromDomain(false, []string{}, "@")

	jSpan := otel_jaeger.SpanToJaegerProto(span, scope)
	jSpan.Process = otel_jaeger.ResourceToJaegerProtoProcess(resource)

	convertedSpan := fromDomain.FromDomainEmbedProcess(jSpan)
	return json.Marshal(convertedSpan)
}

func (m *encodeJaegerModel) encodeLog(resource pcommon.Resource, record plog.LogRecord) ([]byte, error) {
	//do nothing
	return nil, nil
}
