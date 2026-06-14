// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsecsattributesprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/awsecsattributesprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/consumer/xconsumer"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.uber.org/zap"
)

type profilesProcessor struct {
	*ecsCore
	next xconsumer.Profiles
}

func newProfilesProcessor(logger *zap.Logger, cfg *Config, next xconsumer.Profiles, endpoints endpointsFn) *profilesProcessor {
	return &profilesProcessor{ecsCore: newCore(logger, cfg, endpoints), next: next}
}

func (p *profilesProcessor) ConsumeProfiles(ctx context.Context, pd pprofile.Profiles) error {
	rps := pd.ResourceProfiles()
	for i := range rps.Len() {
		p.enrichResource(ctx, rps.At(i).Resource())
	}
	return p.next.ConsumeProfiles(ctx, pd)
}
