// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package skywalkingexporter

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"google.golang.org/grpc"
	v3 "skywalking.apache.org/repo/goapi/collect/common/v3"
	metricpb "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
	logpb "skywalking.apache.org/repo/goapi/collect/logging/v3"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
)

func TestSwExporter(t *testing.T) {
	server, addr, handler := initializeGRPCTestServer(t, grpc.MaxConcurrentStreams(10))
	tt := &Config{
		NumStreams: 10,
		GRPCClientSettings: configgrpc.GRPCClientSettings{
			Endpoint: addr.String(),
			TLSSetting: configtls.TLSClientSetting{
				Insecure: true,
			},
		},
	}

	oce := newLogsExporter(context.Background(), tt, componenttest.NewNopTelemetrySettings())
	got, err := exporterhelper.NewLogsExporter(
		context.Background(),
		exportertest.NewNopCreateSettings(),
		tt,
		oce.pushLogs,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		exporterhelper.WithRetry(tt.BackOffConfig),
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

	w1 := &sync.WaitGroup{}
	var i int64
	for i = 0; i < 200; i++ {
		w1.Add(1)
		go func() {
			defer w1.Done()
			l := testdata.GenerateLogsOneLogRecordNoResource()
			l.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().SetInt(0)
			e := got.ConsumeLogs(context.Background(), l)
			assert.NoError(t, e)
		}()
	}
	w1.Wait()
	var logs []*logpb.LogData
	for i := 0; i < 200; i++ {
		logs = append(logs, <-handler.logChan)
	}
	assert.Equal(t, 200, len(logs))
	assert.Equal(t, 10, len(oce.logsClients))

	// when grpc server stops
	server.Stop()
	w2 := &sync.WaitGroup{}
	for i = 0; i < 200; i++ {
		w2.Add(1)
		go func() {
			defer w2.Done()
			l := testdata.GenerateLogsOneLogRecordNoResource()
			l.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().SetInt(0)
			e := got.ConsumeLogs(context.Background(), l)
			if e != nil {
				return
			}
		}()
	}
	w2.Wait()
	assert.Equal(t, 10, len(oce.logsClients))

	server, addr, handler2 := initializeGRPCTestServerMetric(t, grpc.MaxConcurrentStreams(10))
	tt = &Config{
		NumStreams: 10,
		GRPCClientSettings: configgrpc.GRPCClientSettings{
			Endpoint: addr.String(),
			TLSSetting: configtls.TLSClientSetting{
				Insecure: true,
			},
		},
	}

	oce = newMetricsExporter(context.Background(), tt, componenttest.NewNopTelemetrySettings())
	got2, err2 := exporterhelper.NewMetricsExporter(
		context.Background(),
		exportertest.NewNopCreateSettings(),
		tt,
		oce.pushMetrics,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		exporterhelper.WithRetry(tt.BackOffConfig),
		exporterhelper.WithQueue(tt.QueueSettings),
		exporterhelper.WithTimeout(tt.TimeoutSettings),
		exporterhelper.WithStart(oce.start),
		exporterhelper.WithShutdown(oce.shutdown),
	)
	assert.NoError(t, err2)
	assert.NotNil(t, got2)

	t.Cleanup(func() {
		require.NoError(t, got2.Shutdown(context.Background()))
	})

	err = got2.Start(context.Background(), componenttest.NewNopHost())

	assert.NoError(t, err)

	w1 = &sync.WaitGroup{}
	for i = 0; i < 200; i++ {
		w1.Add(1)
		go func() {
			defer w1.Done()
			l := testdata.GenerateMetricsOneMetric()
			e := got2.ConsumeMetrics(context.Background(), l)
			assert.NoError(t, e)
		}()
	}
	w1.Wait()
	var metrics []*metricpb.MeterDataCollection
	for i := 0; i < 200; i++ {
		metrics = append(metrics, <-handler2.metricChan)
	}
	assert.Equal(t, 200, len(metrics))
	assert.Equal(t, 10, len(oce.metricsClients))

	// when grpc server stops
	server.Stop()
	w3 := &sync.WaitGroup{}
	for i = 0; i < 200; i++ {
		w3.Add(1)
		go func() {
			defer w3.Done()
			l := testdata.GenerateMetricsOneMetric()
			e := got2.ConsumeMetrics(context.Background(), l)
			if e != nil {
				return
			}
		}()
	}
	w3.Wait()
	assert.Equal(t, 10, len(oce.metricsClients))
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

func initializeGRPCTestServerMetric(t *testing.T, opts ...grpc.ServerOption) (*grpc.Server, net.Addr, *mockMetricHandler) {
	server := grpc.NewServer(opts...)
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	m := &mockMetricHandler{
		metricChan: make(chan *metricpb.MeterDataCollection, 200),
	}
	metricpb.RegisterMeterReportServiceServer(
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
		if errors.Is(err, io.EOF) {
			return stream.SendAndClose(&v3.Commands{})
		}
		if err == nil {
			h.logChan <- r
		}
	}
}

type mockMetricHandler struct {
	metricChan chan *metricpb.MeterDataCollection
	metricpb.UnimplementedMeterReportServiceServer
}

func (h *mockMetricHandler) CollectBatch(stream metricpb.MeterReportService_CollectBatchServer) error {
	for {
		r, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return stream.SendAndClose(&v3.Commands{})
		}
		if err == nil {
			h.metricChan <- r
		}
	}
}
