// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsecsattributesprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/awsecsattributesprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/consumer/xconsumer"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper/xprocessorhelper"
	"go.opentelemetry.io/collector/processor/xprocessor"
)

// newProfilesProcessor builds a profiles processor that enriches each resource
// with ECS metadata. xprocessorhelper wraps the enrichment function with the
// standard capabilities and Start/Shutdown lifecycle used across the collector.
func newProfilesProcessor(ctx context.Context, set processor.Settings, cfg *Config, next xconsumer.Profiles, endpoints endpointsFn) (xprocessor.Profiles, error) {
	core, err := newCore(set.Logger, cfg, endpoints)
	if err != nil {
		return nil, err
	}
	return xprocessorhelper.NewProfiles(
		ctx, set, cfg, next,
		core.processProfiles,
		xprocessorhelper.WithCapabilities(core.Capabilities()),
		xprocessorhelper.WithStart(core.Start),
		xprocessorhelper.WithShutdown(core.Shutdown),
	)
}

func (e *ecsCore) processProfiles(ctx context.Context, pd pprofile.Profiles) (pprofile.Profiles, error) {
	rps := pd.ResourceProfiles()
	for i := range rps.Len() {
		e.enrichResource(ctx, rps.At(i).Resource())
	}
	return pd, nil
}
