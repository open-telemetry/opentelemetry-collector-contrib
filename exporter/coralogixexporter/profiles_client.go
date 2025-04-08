// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package coralogixexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/coralogixexporter"

import (
	"context"
	"errors"
	"fmt"
	"runtime"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/pprofile/pprofileotlp"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func newProfilesExporter(cfg component.Config, set exporter.Settings) (*profilesExporter, error) {
	oCfg := cfg.(*Config)

	if isEmpty(oCfg.Domain) && isEmpty(oCfg.Profiles.Endpoint) {
		return nil, errors.New("coralogix exporter config requires `domain` or `profiles.endpoint` configuration")
	}

	userAgent := fmt.Sprintf("%s/%s (%s/%s)",
		set.BuildInfo.Description, set.BuildInfo.Version, runtime.GOOS, runtime.GOARCH)

	return &profilesExporter{
		config:    oCfg,
		settings:  set.TelemetrySettings,
		userAgent: userAgent,
	}, nil
}

type profilesExporter struct {
	// Input configuration.
	config *Config

	profilesExporter pprofileotlp.GRPCClient
	clientConn       *grpc.ClientConn
	callOptions      []grpc.CallOption

	settings component.TelemetrySettings

	// Default user-agent header.
	userAgent string
}

func (e *profilesExporter) start(ctx context.Context, host component.Host) (err error) {
	switch {
	case !isEmpty(e.config.Profiles.Endpoint):
		if e.clientConn, err = e.config.Profiles.ToClientConn(ctx, host, e.settings, configgrpc.WithGrpcDialOption(grpc.WithUserAgent(e.userAgent))); err != nil {
			return err
		}
	case !isEmpty(e.config.Domain):
		if e.clientConn, err = e.config.getDomainGrpcSettings().ToClientConn(ctx, host, e.settings, configgrpc.WithGrpcDialOption(grpc.WithUserAgent(e.userAgent))); err != nil {
			return err
		}
	}

	e.profilesExporter = pprofileotlp.NewGRPCClient(e.clientConn)
	if e.config.Profiles.Headers == nil {
		e.config.Profiles.Headers = make(map[string]configopaque.String)
	}
	e.config.Profiles.Headers["Authorization"] = configopaque.String("Bearer " + string(e.config.PrivateKey))

	e.callOptions = []grpc.CallOption{
		grpc.WaitForReady(e.config.Profiles.WaitForReady),
	}

	return
}

func (e *profilesExporter) pushProfiles(ctx context.Context, md pprofile.Profiles) error {
	rss := md.ResourceProfiles()
	for i := 0; i < rss.Len(); i++ {
		resourceProfile := rss.At(i)
		appName, subsystem := e.config.getMetadataFromResource(resourceProfile.Resource())
		resourceProfile.Resource().Attributes().PutStr(cxAppNameAttrName, appName)
		resourceProfile.Resource().Attributes().PutStr(cxSubsystemNameAttrName, subsystem)
	}

	resp, err := e.profilesExporter.Export(e.enhanceContext(ctx), pprofileotlp.NewExportRequestFromProfiles(md), e.callOptions...)
	if err != nil {
		return processError(err)
	}

	partialSuccess := resp.PartialSuccess()
	if partialSuccess.ErrorMessage() != "" || partialSuccess.RejectedProfiles() != 0 {
		e.settings.Logger.Error("Partial success response from Coralogix",
			zap.String("message", partialSuccess.ErrorMessage()),
			zap.Int64("rejected_profiles", partialSuccess.RejectedProfiles()),
		)
	}
	return nil
}

func (e *profilesExporter) shutdown(context.Context) error {
	if e.clientConn == nil {
		return nil
	}
	return e.clientConn.Close()
}

func (e *profilesExporter) enhanceContext(ctx context.Context) context.Context {
	md := metadata.New(nil)
	for k, v := range e.config.Profiles.Headers {
		md.Set(k, string(v))
	}
	return metadata.NewOutgoingContext(ctx, md)
}
