// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package websocketviewerextension

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"sort"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

type wsproc struct {
	Name  string
	Port  int
	Limit rate.Limit
}

type WebSocketViewerExtension struct {
	wpr    *wsProcRegistry
	logger *zap.Logger
	server *http.Server
}

func NewWebSocketViewerExtension(logger *zap.Logger, port int) *WebSocketViewerExtension {
	wpr := &wsProcRegistry{}
	mux := http.NewServeMux()
	htmlDir := filepath.Join("extension", "websocketviewerextension", "http")
	mux.Handle("/", http.FileServer(http.Dir(htmlDir)))
	mux.Handle("/processors", procInfoHandler{
		logger: logger,
		wpr:    wpr,
	})
	logger.Info("setting up http server", zap.Int("port", port))
	return &WebSocketViewerExtension{
		logger: logger,
		wpr:    wpr,
		server: &http.Server{
			Addr:    fmt.Sprintf(":%d", port),
			Handler: mux,
		},
	}
}

func (s *WebSocketViewerExtension) Start(ctx context.Context, host component.Host) error {
	go s.startServer()
	return nil
}

func (s *WebSocketViewerExtension) startServer() {
	s.logger.Debug("starting server")
	err := s.server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		s.logger.Error("server error", zap.Error(err))
	}
}

func (s *WebSocketViewerExtension) RegisterWebsocketProcessor(name string, port int, limit rate.Limit) {
	s.logger.Debug(
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

func (s *WebSocketViewerExtension) Shutdown(ctx context.Context) error {
	return nil
}

type wsProcRegistry struct {
	mu sync.Mutex
	a  []wsproc
}

func (r *wsProcRegistry) add(p wsproc) {
	r.mu.Lock()
	r.a = append(r.a, p)
	sort.Slice(r.a, func(i, j int) bool {
		return r.a[i].Port < r.a[j].Port
	})
	r.mu.Unlock()
}

func (r *wsProcRegistry) toJSON() ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return json.Marshal(r.a)
}
