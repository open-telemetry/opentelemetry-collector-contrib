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

package skywalkingexporter

import (
	"context"
	"io"
	"net"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"google.golang.org/grpc"
	v3 "skywalking.apache.org/repo/goapi/collect/common/v3"
	logpb "skywalking.apache.org/repo/goapi/collect/logging/v3"
)

func TestSwExporter(t *testing.T) {
	server, addr, handler := initializeGRPCTestServer(t, grpc.MaxConcurrentStreams(10))
	tt := &Config{
		NumStreams:       10,
		ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
		GRPCClientSettings: configgrpc.GRPCClientSettings{
			Endpoint: addr.String(),
			TLSSetting: &configtls.TLSClientSetting{
				Insecure: true,
			},
		},
	}

	oce, err := newExporter(context.Background(), tt)
	assert.NoError(t, err)

	got, err := exporterhelper.NewLogsExporter(
		tt,
		componenttest.NewNopExporterCreateSettings(),
		oce.pushLogs,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		exporterhelper.WithRetry(tt.RetrySettings),
		exporterhelper.WithQueue(tt.QueueSettings),
		exporterhelper.WithTimeout(tt.TimeoutSettings),
		exporterhelper.WithStart(oce.start),
		exporterhelper.WithShutdown(oce.shutdown),
	)
	assert.NoError(t, err)
	assert.NotNil(t, got)

	t.Cleanup(func() {
		require.NoError(t, got.Shutdown(context.Background()))
	})

	err = got.Start(context.Background(), componenttest.NewNopHost())

	assert.NoError(t, err)
	var i int64
	for i = 0; i < 200; i++ {
		go func(i int64) {
			l := testdata.GenerateLogsOneLogRecordNoResource()
			l.ResourceLogs().At(0).InstrumentationLibraryLogs().At(0).Logs().At(0).Body().SetIntVal(i)
			err = got.ConsumeLogs(context.Background(), l)
			assert.NoError(t, err)
		}(i)
	}

	logs := make([]*logpb.LogData, 0)
	for i := 0; i < 200; i++ {
		logs = append(logs, <-handler.logChan)
	}
	assert.Equal(t, 200, len(logs))
	assert.Equal(t, 10, len(oce.logsClients))

	//when grpc server stops
	server.Stop()
	for i = 0; i < 200; i++ {
		go func(i int64) {
			l := testdata.GenerateLogsOneLogRecordNoResource()
			l.ResourceLogs().At(0).InstrumentationLibraryLogs().At(0).Logs().At(0).Body().SetIntVal(i)
			err = got.ConsumeLogs(context.Background(), l)
			assert.Error(t, err)
		}(i)
	}
	assert.Equal(t, 10, len(oce.logsClients))
}

func initializeGRPCTestServer(t *testing.T, opts ...grpc.ServerOption) (*grpc.Server, net.Addr, *mockLogHandler) {
	server := grpc.NewServer(opts...)
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	m := &mockLogHandler{
		logChan: make(chan *logpb.LogData, 200),
	}
	logpb.RegisterLogReportServiceServer(
		server,
		m,
	)
	go func() {
		require.NoError(t, server.Serve(lis))
	}()
	return server, lis.Addr(), m
}

type mockLogHandler struct {
	logChan chan *logpb.LogData
	logpb.UnimplementedLogReportServiceServer
}

func (h *mockLogHandler) Collect(stream logpb.LogReportService_CollectServer) error {
	for {
		r, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&v3.Commands{})
		}
		if err == nil {
			h.logChan <- r
		}
	}
}
