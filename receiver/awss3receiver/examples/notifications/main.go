// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/open-telemetry/opamp-go/server"
	"github.com/open-telemetry/opamp-go/server/types"
	"go.opentelemetry.io/collector/pdata/plog"
)

const (
	openTelemetryCollectorReceiverAWSS3 = "org.opentelemetry.collector.receiver.awss3"
)

type TimeBasedIngestStatus struct {
	TelemetryType  string
	IngestStatus   string
	StartTime      time.Time
	EndTime        time.Time
	IngestTime     time.Time
	FailureMessage string
}

type ProgressServer struct {
	server server.OpAMPServer
}

func main() {
	progressServer := newProgressServer()
	err := progressServer.Start()
	if err != nil {
		panic(err)
	}

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	<-interrupt

	progressServer.Stop()
}

func newProgressServer() *ProgressServer {
	logger := &Logger{
		log.New(
			log.Default().Writer(),
			"[OPAMP] ",
			log.Default().Flags()|log.Lmsgprefix|log.Lmicroseconds,
		),
	}
	return &ProgressServer{
		server: server.New(logger),
	}
}

func (p *ProgressServer) Start() error {
	return p.server.Start(server.StartSettings{
		Settings: server.Settings{
			Callbacks: server.CallbacksStruct{
				OnConnectingFunc: p.onConnecting,
			},

			EnableCompression:  false,
			CustomCapabilities: []string{openTelemetryCollectorReceiverAWSS3},
		},
		ListenEndpoint: "localhost:8080",
		ListenPath:     "",
		TLSConfig:      nil,
		HTTPMiddleware: nil,
	})
}

func (p *ProgressServer) Stop() {
	p.server.Stop(context.Background())
}

func (p *ProgressServer) onConnecting(request *http.Request) types.ConnectionResponse {
	fmt.Println("OnConnecting")
	return types.ConnectionResponse{
		Accept: true,
		ConnectionCallbacks: server.ConnectionCallbacksStruct{
			OnMessageFunc: p.onMessage,
		},
	}
}

func (p *ProgressServer) onMessage(_ context.Context, _ types.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {
	response := &protobufs.ServerToAgent{
		InstanceUid: message.InstanceUid,
	}

	if message.CustomCapabilities != nil {
		capabilities := make([]string, 0)
		for _, capability := range message.CustomCapabilities.Capabilities {
			if capability == "org.opentelemetry.collector.receiver.awss3" {

				capabilities = append(capabilities, capability)
			}
		}
		if len(capabilities) == 0 {
			fmt.Println("🛑 - Agent does not support AWS S3 receiver progress")
		} else {
			fmt.Println("✅ - Agent supports AWS S3 receiver progress")
		}

		response.CustomCapabilities = &protobufs.CustomCapabilities{
			Capabilities: capabilities,
		}
	}

	if message.CustomMessage != nil && message.CustomMessage.Capability == openTelemetryCollectorReceiverAWSS3 {
		if message.CustomMessage.Type == "TimeBasedIngestStatus" {
			status, err := extractStatusMessage(message.CustomMessage.Data)
			if err != nil {
				fmt.Println("💣 - Error unmarshalling custom message data", err)
			} else {
				switch status.IngestStatus {
				case "completed":
					fmt.Println("🎉 - Ingest complete")
				case "failed":
					fmt.Println("🚨 - Ingest failed:", status.FailureMessage)
				case "ingesting":
					done := status.IngestTime.Sub(status.StartTime)
					left := status.EndTime.Sub(status.IngestTime)
					fmt.Printf("🚀 - Ingesting %s done, %s left (current %s)\n", done, left, status.IngestTime)
				}
			}
		}
	}

	return response
}

func extractStatusMessage(bytes []byte) (TimeBasedIngestStatus, error) {
	unmarshaler := plog.ProtoUnmarshaler{}
	logs, err := unmarshaler.UnmarshalLogs(bytes)
	if err != nil {
		return TimeBasedIngestStatus{}, err
	}
	rlogs := logs.ResourceLogs()
	if rlogs.Len() == 0 {
		return TimeBasedIngestStatus{}, fmt.Errorf("no resource logs found")
	}
	slogs := rlogs.At(0).ScopeLogs()
	if slogs.Len() == 0 {
		return TimeBasedIngestStatus{}, fmt.Errorf("no scope logs found")
	}
	lr := slogs.At(0).LogRecords()
	if lr.Len() == 0 {
		return TimeBasedIngestStatus{}, fmt.Errorf("no log records found")
	}
	log := lr.At(0)
	status := TimeBasedIngestStatus{}
	if log.Body().Str() != "status" {
		return TimeBasedIngestStatus{}, fmt.Errorf("not a status message")
	}
	attrs := log.Attributes()
	if v, ok := attrs.Get("telemetry_type"); ok {
		status.TelemetryType = v.Str()
	}
	if v, ok := attrs.Get("ingest_status"); ok {
		status.IngestStatus = v.Str()
	}
	if v, ok := attrs.Get("start_time"); ok {
		if t, err := time.Parse(time.RFC3339, v.Str()); err == nil {
			status.StartTime = t
		} else {
			return TimeBasedIngestStatus{}, fmt.Errorf("bad time for start_time: %q", v.Str())
		}
	}
	if v, ok := attrs.Get("end_time"); ok {
		if t, err := time.Parse(time.RFC3339, v.Str()); err == nil {
			status.EndTime = t
		} else {
			return TimeBasedIngestStatus{}, fmt.Errorf("bad time for end_time: %q", v.Str())
		}
	}
	if v, ok := attrs.Get("ingest_time"); ok {
		if t, err := time.Parse(time.RFC3339, v.Str()); err == nil {
			status.IngestTime = t
		} else {
			return TimeBasedIngestStatus{}, fmt.Errorf("bad time for ingest_time: %q", v.Str())
		}
	}
	if v, ok := attrs.Get("failure_message"); ok {
		status.FailureMessage = v.Str()
	}
	return status, nil
}
