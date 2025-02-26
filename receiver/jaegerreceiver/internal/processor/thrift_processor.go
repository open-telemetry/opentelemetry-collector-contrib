// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package processor

import (
	"context"
	"sync"

	apacheThrift "github.com/apache/thrift/lib/go/thrift"
	"github.com/jaegertracing/jaeger-idl/thrift-gen/agent"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerreceiver/internal/udpserver"
)

// ThriftProcessor is a processor that handles Thrift over UDP
type ThriftProcessor struct {
	server          *udpserver.UDPServer
	protocolFactory apacheThrift.TProtocolFactory
	handler         agent.Agent
	logger          *zap.Logger
	stopCh          chan struct{}
	stopOnce        sync.Once
}

// NewThriftProcessor creates a new ThriftProcessor
func NewThriftProcessor(
	endpoint string,
	maxPacketSize int,
	protocolFactory apacheThrift.TProtocolFactory,
	handler agent.Agent,
	logger *zap.Logger,
) (*ThriftProcessor, error) {
	processor := &ThriftProcessor{
		protocolFactory: protocolFactory,
		handler:         handler,
		logger:          logger,
		stopCh:          make(chan struct{}),
	}

	// Create a packet handler that will process the UDP packets
	packetHandler := &thriftPacketHandler{
		protocolFactory: protocolFactory,
		handler:         handler,
		logger:          logger,
	}

	// Create the UDP server
	server, err := udpserver.New(endpoint, maxPacketSize, packetHandler, logger)
	if err != nil {
		return nil, err
	}
	processor.server = server

	return processor, nil
}

// Start starts the processor
func (p *ThriftProcessor) Start() error {
	return p.server.Start()
}

// Stop stops the processor
func (p *ThriftProcessor) Stop() error {
	p.stopOnce.Do(func() {
		close(p.stopCh)
		p.server.Stop()
	})
	return nil
}

// Done returns a channel that's closed when the processor has stopped
func (p *ThriftProcessor) Done() <-chan struct{} {
	return p.stopCh
}

// thriftPacketHandler implements the PacketHandler interface for processing Thrift packets
type thriftPacketHandler struct {
	protocolFactory apacheThrift.TProtocolFactory
	handler         agent.Agent
	logger          *zap.Logger
}

// Handle processes a UDP packet containing a Thrift message
func (h *thriftPacketHandler) Handle(ctx context.Context, packet []byte) error {
	transport := apacheThrift.NewTMemoryBufferLen(len(packet))
	if _, err := transport.Write(packet); err != nil {
		h.logger.Error("Failed to write to memory buffer", zap.Error(err))
		return err
	}

	protocol := h.protocolFactory.GetProtocol(transport)
	processor := agent.NewAgentProcessor(h.handler)

	ok, err := processor.Process(ctx, protocol, protocol)
	if !ok {
		return err
	}
	return nil
}
