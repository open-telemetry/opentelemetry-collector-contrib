// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package benchmarks compares the throughput and latency of the gnet transport
// (gnetreceiver + gnetexporter) against the standard OTLP/gRPC transport.
//
// Run:
//
//	go test -bench=. -benchtime=10s -benchmem ./benchmarks/
//
// For flamegraph profiling:
//
//	go test -bench=BenchmarkGnet -cpuprofile=cpu.pprof ./benchmarks/
//	go tool pprof -http=:6060 cpu.pprof
package benchmarks

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/panjf2000/gnet/v2"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	collogpb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func makeTracePayload(n int) []byte {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	for i := 0; i < n; i++ {
		s := ss.Spans().AppendEmpty()
		s.SetName(fmt.Sprintf("span-%d", i))
		s.SetTraceID([16]byte{byte(i)})
		s.SetSpanID([8]byte{byte(i)})
	}
	m := &ptrace.ProtoMarshaler{}
	b, _ := m.MarshalTraces(td)
	return b
}

func makeLogPayload(n int) []byte {
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	for i := 0; i < n; i++ {
		lr := sl.LogRecords().AppendEmpty()
		lr.Body().SetStr(fmt.Sprintf("log line %d", i))
	}
	m := &plog.ProtoMarshaler{}
	b, _ := m.MarshalLogs(ld)
	return b
}

func buildFrame(signal byte, payload []byte) []byte {
	frame := make([]byte, 5+len(payload))
	frame[0] = signal
	binary.BigEndian.PutUint32(frame[1:5], uint32(len(payload)))
	copy(frame[5:], payload)
	return frame
}

// ── gnet benchmark ────────────────────────────────────────────────────────────

type devNullHandler struct {
	gnet.BuiltinEventEngine
	received atomic.Int64
}

func (h *devNullHandler) OnTraffic(c gnet.Conn) gnet.Action {
	for {
		if c.InboundBuffered() < 5 {
			return gnet.None
		}
		hdr, _ := c.Peek(5)
		payloadLen := int(binary.BigEndian.Uint32(hdr[1:5]))
		if c.InboundBuffered() < 5+payloadLen {
			return gnet.None
		}
		_, _ = c.Discard(5)
		buf := make([]byte, payloadLen)
		_, _ = c.Read(buf)
		h.received.Add(1)
	}
}

func startGnetServer(t testing.TB) string {
	// Find a free port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()

	h := &devNullHandler{}
	go func() {
		_ = gnet.Run(h, "tcp://"+addr, gnet.WithNumEventLoop(1))
	}()
	time.Sleep(80 * time.Millisecond) // wait for server
	return addr
}

func BenchmarkGnetTraces(b *testing.B) {
	addr := startGnetServer(b)
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		b.Fatal(err)
	}
	defer conn.Close()

	frame := buildFrame(0x01, makeTracePayload(100))
	b.SetBytes(int64(len(frame)))
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := conn.Write(frame); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGnetLogs(b *testing.B) {
	addr := startGnetServer(b)
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		b.Fatal(err)
	}
	defer conn.Close()

	frame := buildFrame(0x03, makeLogPayload(100))
	b.SetBytes(int64(len(frame)))
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := conn.Write(frame); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGnetParallel(b *testing.B) {
	addr := startGnetServer(b)
	frame := buildFrame(0x01, makeTracePayload(100))
	b.SetBytes(int64(len(frame)))
	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
		if err != nil {
			b.Error(err)
			return
		}
		defer conn.Close()
		for pb.Next() {
			if _, err := conn.Write(frame); err != nil {
				b.Error(err)
				return
			}
		}
	})
}

// ── gRPC benchmark ────────────────────────────────────────────────────────────

type grpcTraceDevNull struct {
	coltracepb.UnimplementedTraceServiceServer
	received atomic.Int64
}

func (g *grpcTraceDevNull) Export(_ context.Context, _ *coltracepb.ExportTraceServiceRequest) (*coltracepb.ExportTraceServiceResponse, error) {
	g.received.Add(1)
	return &coltracepb.ExportTraceServiceResponse{}, nil
}

type grpcLogDevNull struct {
	collogpb.UnimplementedLogsServiceServer
	received atomic.Int64
}

func (g *grpcLogDevNull) Export(_ context.Context, _ *collogpb.ExportLogsServiceRequest) (*collogpb.ExportLogsServiceResponse, error) {
	g.received.Add(1)
	return &collogpb.ExportLogsServiceResponse{}, nil
}

func startGRPCServer(t testing.TB) (addr string, stop func()) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer()
	coltracepb.RegisterTraceServiceServer(srv, &grpcTraceDevNull{})
	collogpb.RegisterLogsServiceServer(srv, &grpcLogDevNull{})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = srv.Serve(ln)
	}()
	return ln.Addr().String(), func() {
		srv.GracefulStop()
		wg.Wait()
	}
}

func BenchmarkGRPCTraces(b *testing.B) {
	addr, stop := startGRPCServer(b)
	defer stop()

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		b.Fatal(err)
	}
	defer conn.Close()

	client := coltracepb.NewTraceServiceClient(conn)
	req := &coltracepb.ExportTraceServiceRequest{}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := client.Export(context.Background(), req); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGRPCParallel(b *testing.B) {
	addr, stop := startGRPCServer(b)
	defer stop()

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		b.Fatal(err)
	}
	defer conn.Close()

	client := coltracepb.NewTraceServiceClient(conn)
	req := &coltracepb.ExportTraceServiceRequest{}
	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := client.Export(context.Background(), req); err != nil {
				b.Error(err)
				return
			}
		}
	})
}
