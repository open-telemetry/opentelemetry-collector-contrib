// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package udppoller

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"

	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/obsreport"
	"go.uber.org/zap"

	awsxray "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray"
	recvErr "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/errors"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/socketconn"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/tracesegment"
)

const (
	// Transport is the network transport protocol used
	// by the poller
	Transport = "udp"

	// size of the buffer used by each poller.
	// https://github.com/aws/aws-xray-daemon/blob/master/pkg/cfg/cfg.go#L182
	// https://github.com/aws/aws-xray-daemon/blob/master/cmd/tracing/daemon.go#L171
	pollerBufferSizeKB = 64 * 1024

	// the size of the channel between the UDP poller
	// and OT consumer
	segChanSize = 30
)

// Poller represents one or more goroutines that are
// polling from a UDP socket
type Poller interface {
	SegmentsChan() <-chan RawSegment
	Start(receiverLongTermCtx context.Context)
	Close() error
}

// RawSegment represents a raw X-Ray segment document.
type RawSegment struct {
	// Payload is the raw bytes that represent one X-Ray segment.
	Payload []byte
	// Ctx is the short-lived context created per raw segment received
	Ctx context.Context
}

// Config represents the configurations needed to
// start the UDP poller
type Config struct {
	ReceiverID         config.ComponentID
	Transport          string
	Endpoint           string
	NumOfPollerToStart int
}

type poller struct {
	receiverID           config.ComponentID
	udpSock              socketconn.SocketConn
	logger               *zap.Logger
	wg                   sync.WaitGroup
	receiverLongLivedCtx context.Context
	maxPollerCount       int
	// closing this channel will shutdown all goroutines
	// within this poller
	shutDown chan struct{}

	// all segments read by the poller will be sent to this channel
	segChan chan RawSegment

	obsrecv *obsreport.Receiver
}

// New creates a new UDP poller
func New(cfg *Config, logger *zap.Logger) (Poller, error) {
	if cfg.Transport != Transport {
		return nil, fmt.Errorf(
			"X-Ray receiver only supports ingesting spans through UDP, provided: %s",
			cfg.Transport,
		)
	}

	addr, err := net.ResolveUDPAddr(Transport, cfg.Endpoint)
	if err != nil {
		return nil, err
	}
	sock, err := net.ListenUDP(Transport, addr)
	if err != nil {
		return nil, err
	}
	logger.Info("Listening on endpoint for X-Ray segments",
		zap.String(Transport, addr.String()))

	return &poller{
		receiverID:     cfg.ReceiverID,
		udpSock:        sock,
		logger:         logger,
		maxPollerCount: cfg.NumOfPollerToStart,
		shutDown:       make(chan struct{}),
		segChan:        make(chan RawSegment, segChanSize),
		obsrecv:        obsreport.NewReceiver(obsreport.ReceiverSettings{ReceiverID: cfg.ReceiverID, Transport: cfg.Transport, LongLivedCtx: true}),
	}, nil
}

func (p *poller) Start(receiverLongTermCtx context.Context) {
	p.receiverLongLivedCtx = receiverLongTermCtx
	for i := 0; i < p.maxPollerCount; i++ {
		p.wg.Add(1)
		go p.poll()
	}
}

func (p *poller) Close() error {
	err := p.udpSock.Close()
	close(p.shutDown)
	p.wg.Wait()

	// inform the consumers of segChan that the poller is stopped
	close(p.segChan)
	return err
}

func (p *poller) SegmentsChan() <-chan RawSegment {
	return p.segChan
}

func (p *poller) read(buf *[]byte) (int, error) {
	bufVal := *buf
	rlen, err := p.udpSock.Read(bufVal)
	if err == nil {
		return rlen, nil
	}
	switch err := err.(type) {
	case net.Error:
		if !err.Temporary() {
			return -1, fmt.Errorf("read from UDP socket: %w", &recvErr.ErrIrrecoverable{Err: err})
		}
	default:
		return 0, fmt.Errorf("read from UDP socket: %w", &recvErr.ErrRecoverable{Err: err})
	}
	return 0, fmt.Errorf("read from UDP socket: %w", &recvErr.ErrRecoverable{Err: err})
}

func (p *poller) poll() {
	defer p.wg.Done()
	buffer := make([]byte, pollerBufferSizeKB)
	var (
		errRecv   *recvErr.ErrRecoverable
		errIrrecv *recvErr.ErrIrrecoverable
	)

	for {
		select {
		case <-p.shutDown:
			return
		default:
			ctx := p.obsrecv.StartTracesOp(p.receiverLongLivedCtx)

			bufPointer := &buffer
			rlen, err := p.read(bufPointer)
			if errors.As(err, &errIrrecv) {
				// TODO: We may want to attempt to shutdown/clean the broken socket and open a new one
				// with the same address
				p.logger.Error("Irrecoverable socket read error. Exiting poller", zap.Error(err))
				p.obsrecv.EndTracesOp(ctx, awsxray.TypeStr, 1, err)
				return
			} else if errors.As(err, &errRecv) {
				p.logger.Error("Recoverable socket read error", zap.Error(err))
				p.obsrecv.EndTracesOp(ctx, awsxray.TypeStr, 1, err)
				continue
			}

			bufMessage := buffer[0:rlen]

			header, body, err := tracesegment.SplitHeaderBody(bufMessage)
			// For now tracesegment.SplitHeaderBody does not return irrecoverable error
			// so we don't check for it
			if errors.As(err, &errRecv) {
				p.logger.Error("Failed to split segment header and body",
					zap.Error(err))
				p.obsrecv.EndTracesOp(ctx, awsxray.TypeStr, 1, err)
				continue
			}

			if len(body) == 0 {
				p.logger.Warn("Missing body",
					zap.String("header format", header.Format),
					zap.Int("header version", header.Version),
				)
				p.obsrecv.EndTracesOp(ctx, awsxray.TypeStr, 1,
					errors.New("dropped span due to missing body that contains segment"))
				continue
			}
			copybody := make([]byte, len(body))
			copy(copybody, body)

			p.segChan <- RawSegment{
				Payload: copybody,
				Ctx:     ctx,
			}
		}
	}
}
