// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"encoding/json"

	"github.com/jaegertracing/jaeger/plugin/storage/es/spanstore/dbmodel"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger"
)

type encodeJaegerModel struct {
}

func (m *encodeJaegerModel) encodeSpan(resource pcommon.Resource, scope pcommon.InstrumentationScope, span ptrace.Span) ([]byte, error) {
	// TODO: make this domain converter configurable, refs: https://www.jaegertracing.io/docs/1.46/cli/#jaeger-all-in-one-elasticsearch
	fromDomain := dbmodel.NewFromDomain(false, []string{}, "@")

	jSpan := jaeger.SpanToJaegerProto(span, scope)
	jSpan.Process = jaeger.ResourceToJaegerProtoProcess(resource)

	convertedSpan := fromDomain.FromDomainEmbedProcess(jSpan)
	return json.Marshal(convertedSpan)
}

func (m *encodeJaegerModel) encodeLog(_ pcommon.Resource, _ plog.LogRecord) ([]byte, error) {
	// do nothing
	return nil, nil
}
