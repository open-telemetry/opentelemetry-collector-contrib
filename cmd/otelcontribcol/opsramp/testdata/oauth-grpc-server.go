// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// TLS OAuth/OTLP fixture for logs_collector_integration_test.py in or-agent-cn-ebpf.
// Build from cmd/otelcontribcol: go build -o <path> ./opsramp/testdata/oauth-grpc-server.go
package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	_ "google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type logsServer struct {
	plogotlp.UnimplementedGRPCServer
	emit func(any)
}

func (s *logsServer) Export(ctx context.Context, request plogotlp.ExportRequest) (plogotlp.ExportResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	if len(md.Get("authorization")) != 1 || len(md.Get("tenantid")) != 1 ||
		len(md.Get("hostname")) != 1 || len(md.Get("agent-version")) != 1 {
		return plogotlp.NewExportResponse(), status.Error(codes.Unauthenticated, "required metadata missing")
	}
	s.emit(map[string]any{"kind": "export", "metadata": md, "count": request.Logs().LogRecordCount()})
	return plogotlp.NewExportResponse(), nil
}

func main() {
	root := os.Args[1]
	events, err := os.Create(filepath.Join(root, "events.jsonl"))
	if err != nil {
		log.Fatal(err)
	}
	defer events.Close()
	var mu sync.Mutex
	emit := func(value any) {
		mu.Lock()
		defer mu.Unlock()
		if err := json.NewEncoder(events).Encode(value); err != nil {
			log.Fatal(err)
		}
	}
	var tokens atomic.Int32
	oauth := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		// Require the same body credentials as or-agent's custom exporter. The
		// OAuth library must retry its initial Basic request with body credentials.
		if r.Method != http.MethodPost || r.URL.Path != "/tenancy/auth/oauth/token" ||
			r.URL.Query().Get("agent") != "true" ||
			r.PostForm.Get("grant_type") != "client_credentials" ||
			r.PostForm.Get("client_id") != "test-client" || r.PostForm.Get("client_secret") != "test-secret" {
			http.Error(w, "invalid client", http.StatusUnauthorized)
			return
		}
		token := fmt.Sprintf("test-token-%d", tokens.Add(1))
		emit(map[string]any{"kind": "token", "token": token})
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"access_token": token, "token_type": "Bearer", "expires_in": 2,
		}); err != nil {
			log.Print(err)
		}
	}))
	defer oauth.Close()
	cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: oauth.Certificate().Raw})
	if err := os.WriteFile(filepath.Join(root, "ca.pem"), cert, 0600); err != nil {
		log.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	server := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{
		Certificates: oauth.TLS.Certificates, MinVersion: tls.VersionTLS12,
	})))
	plogotlp.RegisterGRPCServer(server, &logsServer{emit: emit})
	info, err := json.Marshal(map[string]string{
		"endpoint": listener.Addr().String(), "tokenUrl": oauth.URL + "/tenancy/auth/oauth/token?agent=true",
	})
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ready.json"), info, 0600); err != nil {
		log.Fatal(err)
	}
	if err := server.Serve(listener); err != nil {
		log.Fatal(err)
	}
}
