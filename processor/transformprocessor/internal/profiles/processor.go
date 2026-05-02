// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package profiles // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/profiles"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlprofile"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metadata"
)

type Processor struct {
	contexts []common.ProfilesConsumer
	logger   *zap.Logger
	tracer   trace.Tracer
}

func NewProcessor(contextStatements []common.ContextStatements, errorMode ottl.ErrorMode, settings component.TelemetrySettings, profileFunctions map[string]ottl.Factory[*ottlprofile.TransformContext]) (*Processor, error) {
	pc, err := common.NewProfileParserCollection(settings, common.WithProfileParser(profileFunctions), common.WithProfileErrorMode(errorMode))
	if err != nil {
		return nil, err
	}

	contexts := make([]common.ProfilesConsumer, len(contextStatements))
	var errors error
	for i, cs := range contextStatements {
		context, err := pc.ParseContextStatements(cs)
		if err != nil {
			errors = multierr.Append(errors, err)
		}
		contexts[i] = context
	}

	if errors != nil {
		return nil, errors
	}

	return &Processor{
		contexts: contexts,
		logger:   settings.Logger,
		tracer:   metadata.Tracer(settings),
	}, nil
}

func (p *Processor) ProcessProfiles(ctx context.Context, ld pprofile.Profiles) (pprofile.Profiles, error) {
	ctx, span := p.tracer.Start(ctx, "transform.ProcessProfiles")
	defer span.End()

	for _, c := range p.contexts {
		ctxChild, contextSpan := p.tracer.Start(ctx, "transform.context",
			trace.WithAttributes(attribute.String("transform.context.type", string(c.Context()))))
		err := c.ConsumeProfiles(ctxChild, ld)
		if err != nil {
			contextSpan.RecordError(err)
			contextSpan.End()
			p.logger.Error("failed processing profiles", zap.Error(err))
			span.RecordError(err)
			return ld, err
		}
		contextSpan.End()
	}

	if span.IsRecording() {
		span.SetAttributes(
			attribute.Int("transform.context.count", len(p.contexts)),
			attribute.Int("transform.resource_profiles.count", ld.ResourceProfiles().Len()),
		)
	}

	return ld, nil
}
