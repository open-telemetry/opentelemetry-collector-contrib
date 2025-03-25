// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package faro // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/faro"

import (
	"context"

	faroTypes "github.com/grafana/faro/pkg/go"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

// TranslateToTraces converts faro.Payload into Traces pipeline data
func TranslateToTraces(ctx context.Context, payload faroTypes.Payload) (ptrace.Traces, error) {
	_, span := otel.Tracer("").Start(ctx, "TranslateToTraces")
	defer span.End()

	traces := ptrace.NewTraces()
	if payload.Traces == nil {
		return traces, nil
	}

	resspanCount := payload.Traces.Traces.ResourceSpans().Len()
	span.SetAttributes(attribute.Int("count", resspanCount))
	traces.ResourceSpans().EnsureCapacity(resspanCount)
	for i := 0; i < resspanCount; i++ {
		rs := traces.ResourceSpans().AppendEmpty()
		frs := ptrace.NewResourceSpans()
		payload.Traces.Traces.ResourceSpans().At(i).CopyTo(frs)
		frs.Resource().Attributes().PutStr(string(semconv.ServiceNameKey), payload.Meta.App.Name)
		frs.Resource().Attributes().PutStr(string(semconv.ServiceVersionKey), payload.Meta.App.Version)
		frs.Resource().Attributes().PutStr(string(semconv.DeploymentEnvironmentKey), payload.Meta.App.Environment)

		if payload.Meta.App.Namespace != "" {
			frs.Resource().Attributes().PutStr(string(semconv.ServiceNamespaceKey), payload.Meta.App.Namespace)
		}
		frs.CopyTo(rs)
	}

	return traces, nil
}
