// Copyright 2019, OpenTelemetry Authors
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

package awsxrayreceiver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/obsreport"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/socketconn"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/socketconn/udp"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/tracesegment"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/util"
)

const (
	// number of goroutines polling the UDP socket.
	// https://github.com/aws/aws-xray-daemon/blob/master/pkg/cfg/cfg.go#L184
	maxPollerCount = 2

	// size of the buffer used by each poller.
	// https://github.com/aws/aws-xray-daemon/blob/master/pkg/cfg/cfg.go#L182
	// https://github.com/aws/aws-xray-daemon/blob/master/cmd/tracing/daemon.go#L171
	pollerBufferSizeKB = 64 * 1024

	transport = "udp"
)

// ensure the xrayReceiver implements the TraceReceiver interface
var _ component.TraceReceiver = (*xrayReceiver)(nil)

// xrayReceiver implements the component.TraceReceiver interface for converting
// AWS X-Ray segment document into the OT internal trace format.
type xrayReceiver struct {
	instanceName string
	udpSock      socketconn.SocketConn
	logger       *zap.Logger
	consumer     consumer.TraceConsumer
	wg           sync.WaitGroup
	ctx          context.Context
	startOnce    sync.Once
	stopOnce     sync.Once
}

func newReceiver(config *Config,
	consumer consumer.TraceConsumer,
	logger *zap.Logger) (component.TraceReceiver, error) {

	if consumer == nil {
		return nil, componenterror.ErrNilNextConsumer
	}

	if config.Transport != transport {
		return nil, fmt.Errorf(
			"X-Ray receiver only supports ingesting spans through UDP, provided: %s",
			config.Transport,
		)
	}

	logger.Info("listening on endpoint for X-Ray segments",
		zap.String(transport, config.Endpoint))
	sock, err := udp.New(config.Endpoint)
	if err != nil {
		return nil, err
	}

	return &xrayReceiver{
		instanceName: config.Name(),
		udpSock:      sock,
		logger:       logger,
		consumer:     consumer,
	}, nil
}

func (x *xrayReceiver) Start(ctx context.Context, host component.Host) error {
	// TODO: Might want to pass `host` into read() below to report a fatal error

	var err = componenterror.ErrAlreadyStarted
	x.startOnce.Do(func() {
		x.ctx = obsreport.ReceiverContext(ctx, x.instanceName, transport, "")
		for i := 0; i < maxPollerCount; i++ {
			x.wg.Add(1)
			go x.poll()
		}
		err = nil
	})
	return err
}

func (x *xrayReceiver) Shutdown(_ context.Context) error {
	var err = componenterror.ErrAlreadyStopped
	x.stopOnce.Do(func() {
		x.udpSock.Close()
		x.wg.Wait()
		err = nil
	})
	return err
}

// Reference for this port:
// https://github.com/aws/aws-xray-daemon/blob/master/cmd/tracing/daemon.go#L257
func (x *xrayReceiver) read(buf *[]byte) (int, error) {
	bufVal := *buf
	rlen, err := x.udpSock.Read(bufVal)
	if err == nil {
		return rlen, nil
	}
	switch err := err.(type) {
	case net.Error:
		if !err.Temporary() {
			return -1, fmt.Errorf("read from UDP socket: %w", &errIrrecoverable{err})
		}
	default:
		return 0, fmt.Errorf("read from UDP socket: %w", &errRecoverable{err})
	}
	return 0, fmt.Errorf("read from UDP socket: %w", &errRecoverable{err})
}

// this function references the implementation in:
// https://github.com/aws/aws-xray-daemon/blob/master/cmd/tracing/daemon.go#L275
// However, it intentionally avoids using a buffer pool and just stick to
// a local buffer for simplicity and improve concurrency (because in the orignal
// implementation, the buffer pool is shared between 2 poll() cals executed by
// 2 goroutines, which requires locking/unlocking a sync.Mutex whenever
// a buffer is Get()/Return()). Also, the buffer returns by Get()
// (i.e. https://github.com/aws/aws-xray-daemon/blob/d2331c8c4538e55c237b05306a3cf2e919a41ba9/pkg/bufferpool/bufferpool.go#L28)
// is the same size as the fallBackBuffer
// (https://github.com/aws/aws-xray-daemon/blob/master/cmd/tracing/daemon.go#L277)
func (x *xrayReceiver) poll() {
	defer x.wg.Done()
	separator := []byte(util.ProtocolSeparator)
	buffer := make([]byte, pollerBufferSizeKB)
	splitBuf := make([][]byte, 2)

	var (
		errRecv   *errRecoverable
		errIrrecv *errIrrecoverable
	)

	for {
		// TODO:
		// call https://pkg.go.dev/go.opentelemetry.io/collector@v0.4.1-0.20200622191610-a8db6271f90a/obsreport?tab=doc#StartTraceDataReceiveOp
		// once here and
		// https://pkg.go.dev/go.opentelemetry.io/collector@v0.4.1-0.20200622191610-a8db6271f90a/obsreport?tab=doc#EndTraceDataReceiveOp
		// at corresponding places in the for loop below.
		bufPointer := &buffer
		rlen, err := x.read(bufPointer)
		if err != nil && errors.As(err, &errIrrecv) {
			x.logger.Error("irrecoverable socket read error. Exiting poller", zap.Error(err))
			return
		} else if errors.As(err, &errRecv) {
			x.logger.Error("recoverable socket read error", zap.Error(err))
			continue
		}

		bufMessage := buffer[0:rlen]

		slices := util.SplitHeaderBody(x.logger, &bufMessage, &separator, &splitBuf)
		if len(slices[1]) == 0 {
			x.logger.Warn("Missing header or segment", zap.ByteString("value", slices[0]))
			// TODO: emit metric here to indicate segment rejected
			continue
		}

		header := slices[0]
		// payload := slices[1]
		headerInfo := tracesegment.Header{}
		json.Unmarshal(header, &headerInfo)

		switch headerInfo.IsValid() {
		case true:
		default:
			x.logger.Warn("Invalid header", zap.ByteString("header", header))
			// TODO: emit metric here to indicate segment rejected
			continue
		}

		// TODO: Transform payload to consumer.ConsumeTraceData. For now
		// we are just dropping the ingested X-Ray segments.
		// TODO: use the context generated by obsreport.StartMetricsReceiveOp
		x.consumer.ConsumeTraces(context.Background(), pdata.NewTraces())
		// TODO: emit metrics here to indicate segment received
	}
}
