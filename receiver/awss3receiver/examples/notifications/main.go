// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/open-telemetry/opamp-go/server"
	"github.com/open-telemetry/opamp-go/server/types"
)

const (
	openTelemetryCollectorReceiverAWSS3 = "org.opentelemetry.collector.receiver.awss3"
)

type TimeBasedIngestStatus struct {
	TelemetryType  string    `json:"telemetry_type"`
	IngestStatus   string    `json:"ingest_status"`
	StartTime      time.Time `json:"start_time"`
	EndTime        time.Time `json:"end_time"`
	IngestTime     time.Time `json:"ingest_time"`
	FailureMessage string    `json:"failure_message,omitempty"`
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
	return &ProgressServer{
		server: server.New(nil),
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
			status := &TimeBasedIngestStatus{}
			err := json.Unmarshal(message.CustomMessage.Data, status)
			if err != nil {
				fmt.Println("💣 - Error unmarshalling custom message data", err)
			} else {
				switch status.IngestStatus {
				case "complete":
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
