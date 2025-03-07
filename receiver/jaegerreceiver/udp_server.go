// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jaegerreceiver

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"

	apacheThrift "github.com/apache/thrift/lib/go/thrift"
	"github.com/jaegertracing/jaeger-idl/thrift-gen/agent"
	"github.com/jaegertracing/jaeger-idl/thrift-gen/jaeger"
	"github.com/jaegertracing/jaeger-idl/thrift-gen/zipkincore"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"

	jaegertranslator "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger"
)

type UDPServer struct {
	conn          *net.UDPConn
	maxPacketSize int
	protoFactory  apacheThrift.TProtocolFactory
	processor     *agent.AgentProcessor
	handler       agent.Agent
	logger        *zap.Logger

	running  atomic.Bool
	stopWG   sync.WaitGroup
	stopOnce sync.Once
	stopChan chan struct{}
}

func NewUDPServer(
	endpoint string,
	maxPacketSize int,
	nextConsumer consumer.Traces,
	protoFactory apacheThrift.TProtocolFactory,
	logger *zap.Logger,
) (*UDPServer, error) {
	if endpoint == "" {
		return nil, errors.New("empty endpoint")
	}
	if nextConsumer == nil {
		return nil, errors.New("nil nextConsumer")
	}

	host, port, err := net.SplitHostPort(endpoint)
	if err != nil {
		return nil, err
	}

	addr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(host, port))
	if err != nil {
		return nil, err
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}

	handler := &jaegerThriftHandler{
		nextConsumer: nextConsumer,
		logger:       logger,
	}

	s := &UDPServer{
		conn:          conn,
		maxPacketSize: maxPacketSize,
		protoFactory:  protoFactory,
		processor:     agent.NewAgentProcessor(handler),
		handler:       handler,
		logger:        logger,
		stopChan:      make(chan struct{}),
	}

	return s, nil
}

func (s *UDPServer) Start() error {
	if s.running.Swap(true) {
		return errors.New("UDP server has already started")
	}

	s.stopWG.Add(1)
	go func() {
		defer s.stopWG.Done()
		s.serve()
	}()

	s.logger.Info("Jaeger UDP server started", zap.String("address", s.conn.LocalAddr().String()))
	return nil
}

func (s *UDPServer) serve() {
	buf := make([]byte, s.maxPacketSize)

	for {
		select {
		case <-s.stopChan:
			return
		default:
			n, _, err := s.conn.ReadFromUDP(buf)
			if err != nil {
				if !s.running.Load() {
					return
				}
				s.logger.Error("Failed to read UDP packet", zap.Error(err))
				continue
			}

			s.handleMsg(buf[:n])
		}
	}
}

func (s *UDPServer) handleMsg(msg []byte) {
	transport := apacheThrift.NewTMemoryBufferLen(len(msg))
	if _, err := transport.Write(msg); err != nil {
		s.logger.Error("Cannot write to memory buffer", zap.Error(err))
		return
	}

	protocol := s.protoFactory.GetProtocol(transport)
	ctx := context.Background()

	_, err := s.processor.Process(ctx, protocol, protocol)
	if err != nil {
		s.logger.Error("Failed to process UDP packet", zap.Error(err))
	}
}

func (s *UDPServer) Stop() error {
	var err error
	s.stopOnce.Do(func() {
		s.running.Store(false)
		close(s.stopChan)
		err = s.conn.Close()
		s.stopWG.Wait()
		s.logger.Info("Jaeger UDP server stopped")
	})
	return err
}

func (s *UDPServer) Done() <-chan struct{} {
	return s.stopChan
}

type jaegerThriftHandler struct {
	nextConsumer consumer.Traces
	logger       *zap.Logger
}

func (h *jaegerThriftHandler) EmitZipkinBatch(ctx context.Context, spans []*zipkincore.Span) error {
	return errors.New("unsupported zipkin receiver")
}

func (h *jaegerThriftHandler) EmitBatch(ctx context.Context, batch *jaeger.Batch) error {
	if batch == nil || len(batch.Spans) == 0 {
		return nil
	}

	td, err := jaegertranslator.ThriftToTraces(batch)
	if err != nil {
		h.logger.Error("Failed to convert Jaeger batch to traces", zap.Error(err))
		return err
	}

	return h.nextConsumer.ConsumeTraces(ctx, td)
}
