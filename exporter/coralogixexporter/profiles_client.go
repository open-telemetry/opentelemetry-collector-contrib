// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package coralogixexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/coralogixexporter"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/pprofile/pprofileotlp"
	"go.uber.org/zap"
)

func newProfilesExporter(cfg component.Config, set exporter.Settings) (*profilesExporter, error) {
	oCfg, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config exporter, expect type: %T, got: %T", &Config{}, cfg)
	}

	signalExporter, err := newSignalExporter(oCfg, set, oCfg.Profiles.Endpoint, oCfg.Profiles.Headers)
	if err != nil {
		return nil, err
	}

	return &profilesExporter{
		signalExporter: signalExporter,
	}, nil
}

type profilesExporter struct {
	profilesExporter pprofileotlp.GRPCClient
	*signalExporter
}

func (e *profilesExporter) start(ctx context.Context, host component.Host) (err error) {
	wrapper := &signalConfigWrapper{config: &e.config.Profiles}
	if err := e.startSignalExporter(ctx, host, wrapper); err != nil {
		return err
	}
	e.profilesExporter = pprofileotlp.NewGRPCClient(e.clientConn)
	return nil
}

func (e *profilesExporter) pushProfiles(ctx context.Context, md pprofile.Profiles) error {
	if !e.canSend() {
		return e.rateError.GetError()
	}

	rss := md.ResourceProfiles()
	for i := 0; i < rss.Len(); i++ {
		resourceProfile := rss.At(i)
		appName, subsystem := e.config.getMetadataFromResource(resourceProfile.Resource())
		resourceProfile.Resource().Attributes().PutStr(cxAppNameAttrName, appName)
		resourceProfile.Resource().Attributes().PutStr(cxSubsystemNameAttrName, subsystem)
	}

	resp, err := e.profilesExporter.Export(e.enhanceContext(ctx), pprofileotlp.NewExportRequestFromProfiles(md), e.callOptions...)
	if err != nil {
		return e.processError(err)
	}

	partialSuccess := resp.PartialSuccess()
	if partialSuccess.ErrorMessage() != "" || partialSuccess.RejectedProfiles() != 0 {
		e.settings.Logger.Error("Partial success response from Coralogix",
			zap.String("message", partialSuccess.ErrorMessage()),
			zap.Int64("rejected_profiles", partialSuccess.RejectedProfiles()),
		)
	}
	e.rateError.errorCount.Store(0)
	return nil
}

func (e *profilesExporter) enhanceContext(ctx context.Context) context.Context {
	return e.signalExporter.enhanceContext(ctx)
}
