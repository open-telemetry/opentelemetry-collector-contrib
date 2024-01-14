// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package skywalkingexporter

import (
	"context"
	"errors"
	"io"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"google.golang.org/grpc"
	v3 "skywalking.apache.org/repo/goapi/collect/common/v3"
	logpb "skywalking.apache.org/repo/goapi/collect/logging/v3"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
)

var (
	consumerNum = &atomic.Int32{}
	sumNum      = 10000
)

func TestSkywalking(t *testing.T) {

	test(1, 1, t)
	test(1, 2, t)
	test(1, 3, t)
	test(1, 4, t)
	test(1, 5, t)
	test(1, 10, t)

	println()
	test(2, 1, t)
	test(2, 2, t)
	test(2, 3, t)
	test(2, 4, t)
	test(2, 5, t)
	test(2, 10, t)

	println()
	test(4, 1, t)
	test(4, 2, t)
	test(4, 3, t)
	test(4, 4, t)
	test(4, 5, t)
	test(4, 10, t)

	println()
	test(5, 1, t)
	test(5, 2, t)
	test(5, 3, t)
	test(5, 4, t)
	test(5, 5, t)
	test(5, 10, t)

	println()
	test(10, 1, t)
	test(10, 2, t)
	test(10, 3, t)
	test(10, 4, t)
	test(10, 5, t)
	test(10, 10, t)
	test(10, 15, t)
	test(10, 20, t)
}

func test(nGoroutine int, nStream int, t *testing.T) {
	exporter, server, m := doInit(nStream, t)
	consumerNum.Store(-int32(nStream))
	l := testdata.GenerateLogsOneLogRecordNoResource()
	l.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().SetInt(0)

	for i := 0; i < nStream; i++ {
		err := exporter.pushLogs(context.Background(), l)
		assert.NoError(t, err)
	}

	workers := nGoroutine
	w := &sync.WaitGroup{}
	start := time.Now().UnixMilli()
	for i := 0; i < workers; i++ {
		w.Add(1)
		go func() {
			defer w.Done()
			for i := 0; i < sumNum/workers; i++ {
				err := exporter.pushLogs(context.Background(), l)
				assert.NoError(t, err)
			}
		}()
	}
	w.Wait()
	end := time.Now().UnixMilli()
	print("The number of goroutines:")
	print(nGoroutine)
	print(",  The number of streams:")
	print(nStream)
	print(",  Sent: " + strconv.Itoa(sumNum) + " items (" + strconv.Itoa(sumNum/int(end-start)) + "/millisecond)")
	end = <-m.stopChan
	assert.NotEqual(t, end, -1)
	println(",  Receive: " + strconv.Itoa(sumNum) + " items (" + strconv.Itoa(sumNum/(int(end-start))) + "/millisecond)")

	server.Stop()
	err := exporter.shutdown(context.Background())
	assert.NoError(t, err)
}

func doInit(numStream int, t *testing.T) (*swExporter, *grpc.Server, *mockLogHandler2) {
	server, addr, m := initializeGRPC(grpc.MaxConcurrentStreams(100))
	tt := &Config{
		NumStreams: numStream,
		QueueSettings: exporterhelper.QueueSettings{
			Enabled:      true,
			NumConsumers: 1,
			QueueSize:    1000,
		},
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

	if err != nil {
		t.Errorf("error")
	}

	err = got.Start(context.Background(), componenttest.NewNopHost())
	assert.NoError(t, err)
	return oce, server, m
}

func initializeGRPC(opts ...grpc.ServerOption) (*grpc.Server, net.Addr, *mockLogHandler2) {
	server := grpc.NewServer(opts...)
	lis, _ := net.Listen("tcp", "localhost:0")
	m := &mockLogHandler2{
		stopChan: make(chan int64),
	}
	logpb.RegisterLogReportServiceServer(
		server,
		m,
	)
	go func() {
		err := server.Serve(lis)
		if err != nil {
			return
		}
	}()
	return server, lis.Addr(), m
}

type mockLogHandler2 struct {
	stopChan chan int64
	logpb.UnimplementedLogReportServiceServer
}

func (h *mockLogHandler2) Collect(stream logpb.LogReportService_CollectServer) error {
	for {
		_, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			h.stopChan <- -1
			return stream.SendAndClose(&v3.Commands{})
		}
		if err == nil {
			consumerNum.Add(1)
			if consumerNum.Load() >= int32(sumNum) {
				end := time.Now().UnixMilli()
				h.stopChan <- end
				return nil
			}
		} else {
			err := stream.SendAndClose(&v3.Commands{})
			h.stopChan <- -1
			return err
		}
	}
}
