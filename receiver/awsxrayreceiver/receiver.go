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

package awsxrayreceiver

import (
	"context"
	"fmt"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/obsreport"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/awsxray"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/proxy"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/translator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/udppoller"
)

const (
	// number of goroutines polling the UDP socket.
	// https://github.com/aws/aws-xray-daemon/blob/master/pkg/cfg/cfg.go#L184
	maxPollerCount = 2
)

// xrayReceiver implements the component.TraceReceiver interface for converting
// AWS X-Ray segment document into the OT internal trace format.
type xrayReceiver struct {
	instanceName string
	poller       udppoller.Poller
	server       proxy.Server
	logger       *zap.Logger
	consumer     consumer.TracesConsumer
	longLivedCtx context.Context
	startOnce    sync.Once
	stopOnce     sync.Once
}

func newReceiver(config *Config,
	consumer consumer.TracesConsumer,
	logger *zap.Logger) (component.TraceReceiver, error) {

	if consumer == nil {
		return nil, componenterror.ErrNilNextConsumer
	}

	logger.Info("Going to listen on endpoint for X-Ray segments",
		zap.String(udppoller.Transport, config.Endpoint))
	poller, err := udppoller.New(&udppoller.Config{
		ReceiverInstanceName: config.Name(),
		Transport:            config.Transport,
		Endpoint:             config.Endpoint,
		NumOfPollerToStart:   maxPollerCount,
	}, logger)
	if err != nil {
		return nil, err
	}

	logger.Info("Listening on endpoint for X-Ray segments",
		zap.String(udppoller.Transport, config.Endpoint))

	srv, err := proxy.NewServer(config.ProxyServer, logger)
	if err != nil {
		return nil, err
	}

	return &xrayReceiver{
		instanceName: config.Name(),
		poller:       poller,
		server:       srv,
		logger:       logger,
		consumer:     consumer,
	}, nil
}

func (x *xrayReceiver) Start(ctx context.Context, host component.Host) error {
	// TODO: Might want to pass `host` into read() below to report a fatal error
	var err = componenterror.ErrAlreadyStarted
	x.startOnce.Do(func() {
		x.longLivedCtx = obsreport.ReceiverContext(ctx, x.instanceName, udppoller.Transport, "")
		x.poller.Start(x.longLivedCtx)
		go x.start()
		go x.server.ListenAndServe()
		x.logger.Info("X-Ray TCP proxy server started")
		err = nil
	})
	return err
}

func (x *xrayReceiver) Shutdown(_ context.Context) error {
	var err = componenterror.ErrAlreadyStopped
	x.stopOnce.Do(func() {
		err = nil
		pollerErr := x.poller.Close()
		if pollerErr != nil {
			err = pollerErr
		}

		proxyErr := x.server.Close()
		if proxyErr != nil {
			if err == nil {
				err = proxyErr
			} else {
				err = fmt.Errorf("failed to close proxy: %s: failed to close poller: %s",
					proxyErr.Error(), err.Error())
			}
		}
	})
	return err
}

func (x *xrayReceiver) start() {
	incomingSegments := x.poller.SegmentsChan()
	for seg := range incomingSegments {
		traces, totalSpansCount, err := translator.ToTraces(seg.Payload)
		if err != nil {
			x.logger.Warn("X-Ray segment to OT traces conversion failed", zap.Error(err))
			obsreport.EndTraceDataReceiveOp(seg.Ctx, awsxray.TypeStr, totalSpansCount, err)
			continue
		}

		err = x.consumer.ConsumeTraces(seg.Ctx, *traces)
		if err != nil {
			x.logger.Warn("Trace consumer errored out", zap.Error(err))
			obsreport.EndTraceDataReceiveOp(seg.Ctx, awsxray.TypeStr, totalSpansCount, err)
			continue
		}
		obsreport.EndTraceDataReceiveOp(seg.Ctx, awsxray.TypeStr, totalSpansCount, nil)
	}
}
