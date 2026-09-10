// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pprofreceiver/internal"

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/google/pprof/profile"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer/xconsumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/pprof"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pprofreceiver/internal/metadata"
)

// PushPath is the path the pprof push server listens on.
const PushPath = "/v1/pprof"

// HTTPServer accepts pprof data pushed over HTTP and forwards it to the next consumer.
type HTTPServer struct {
	ServerConfig confighttp.ServerConfig
	Consumer     xconsumer.Profiles
	Settings     receiver.Settings

	server     *http.Server
	shutdownWG sync.WaitGroup
	obsrep     *receiverhelper.ObsReport
}

var _ component.Component = (*HTTPServer)(nil)

func (s *HTTPServer) Start(ctx context.Context, host component.Host) error {
	var err error
	s.obsrep, err = receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             s.Settings.ID,
		Transport:              "http",
		ReceiverCreateSettings: s.Settings,
	})
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.HandleFunc(PushPath, s.handlePush)

	s.server, err = s.ServerConfig.ToServer(ctx, host.GetExtensions(), s.Settings.TelemetrySettings, mux)
	if err != nil {
		return fmt.Errorf("failed to create pprof push http server: %w", err)
	}

	listener, err := s.ServerConfig.ToListener(ctx)
	if err != nil {
		return fmt.Errorf("failed to create pprof push http listener: %w", err)
	}

	s.Settings.Logger.Info("Starting pprof push HTTP server", zap.String("endpoint", s.ServerConfig.NetAddr.Endpoint))
	s.shutdownWG.Go(func() {
		if serveErr := s.server.Serve(listener); !errors.Is(serveErr, http.ErrServerClosed) && serveErr != nil {
			componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(serveErr))
		}
	})
	return nil
}

func (s *HTTPServer) Shutdown(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	err := s.server.Shutdown(ctx)
	s.shutdownWG.Wait()
	return err
}

func (s *HTTPServer) handlePush(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, fmt.Sprintf("method %s not allowed, supported: [POST]", r.Method), http.StatusMethodNotAllowed)
		return
	}

	// confighttp.ToServer transparently handles supported Content-Encodings (e.g. gzip)
	// before we read the body here. Cap uncompressed bodies as well, since the
	// confighttp decompressor only enforces MaxRequestBodySize when a body is
	// actually decoded.
	if s.ServerConfig.MaxRequestBodySize > 0 {
		r.Body = http.MaxBytesReader(w, r.Body, s.ServerConfig.MaxRequestBodySize)
	}
	pprofProfile, err := profile.Parse(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to parse pprof data: %v", err), http.StatusBadRequest)
		return
	}

	profiles, err := pprof.ConvertPprofToProfiles(pprofProfile)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to convert pprof to profiles: %v", err), http.StatusBadRequest)
		return
	}

	// set ScopeName and Version
	for i := 0; i < profiles.ResourceProfiles().Len(); i++ {
		rp := profiles.ResourceProfiles().At(i)
		for j := 0; j < rp.ScopeProfiles().Len(); j++ {
			sp := rp.ScopeProfiles().At(j)

			// Directly setting values without any extra initialization
			sp.Scope().SetName(metadata.ScopeName + "/httpserver")
			sp.Scope().SetVersion(s.Settings.BuildInfo.Version)
		}
	}

	ctx := r.Context()
	if s.obsrep != nil {
		ctx = s.obsrep.StartProfilesOp(ctx)
	}
	consumeErr := s.Consumer.ConsumeProfiles(ctx, *profiles)
	if s.obsrep != nil {
		s.obsrep.EndProfilesOp(ctx, "pprof", profiles.SampleCount(), consumeErr)
	}
	if consumeErr != nil {
		http.Error(w, consumeErr.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
