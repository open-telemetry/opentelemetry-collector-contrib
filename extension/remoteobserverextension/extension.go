// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package remoteobserverextension

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

//go:embed http/*
var httpFS embed.FS

type remoteObserverExtension struct {
	config   *Config
	settings extension.CreateSettings
	wpr      *wsProcRegistry
	server   *http.Server
}

func (s *remoteObserverExtension) Start(_ context.Context, host component.Host) error {

	htmlContent, err := fs.Sub(httpFS, "html")
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(htmlContent)))
	wpr := &wsProcRegistry{}
	mux.Handle("/processors", procInfoHandler{
		wpr: wpr,
	})
	s.server, err = s.config.HTTPServerSettings.ToServer(host, s.settings.TelemetrySettings, mux)
	if err != nil {
		return err
	}

	go func() {
		err := s.server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			host.ReportFatalError(err)
		}
	}()
	return nil
}

func (s *remoteObserverExtension) registerRemoteObserverProcessor(name string, port int, limit rate.Limit) {
	s.settings.Logger.Debug(
		"ws processor registered",
		zap.String("name", name),
		zap.Int("port", port),
		zap.Float64("limit", float64(limit)),
	)
	s.wpr.add(wsproc{
		Name:  name,
		Port:  port,
		Limit: limit,
	})
}

func (s *remoteObserverExtension) Shutdown(_ context.Context) error {
	return nil
}

type wsproc struct {
	Name  string
	Port  int
	Limit rate.Limit
}

type wsProcRegistry struct {
	a []wsproc
}

func (r *wsProcRegistry) add(p wsproc) {
	r.a = append(r.a, p)
}

func (r *wsProcRegistry) toJSON() ([]byte, error) {
	return json.Marshal(r.a)
}
