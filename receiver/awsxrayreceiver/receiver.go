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

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/obsreport"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy"
	awsxray "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/translator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/udppoller"
)

const (
	// number of goroutines polling the UDP socket.
	// https://github.com/aws/aws-xray-daemon/blob/master/pkg/cfg/cfg.go#L184
	maxPollerCount = 2
)

// xrayReceiver implements the component.TracesReceiver interface for converting
// AWS X-Ray segment document into the OT internal trace format.
type xrayReceiver struct {
	instanceID config.ComponentID
	poller     udppoller.Poller
	server     proxy.Server
	settings   component.ReceiverCreateSettings
	consumer   consumer.Traces
	obsrecv    *obsreport.Receiver
}

func newReceiver(config *Config,
	consumer consumer.Traces,
	set component.ReceiverCreateSettings) (component.TracesReceiver, error) {

	if consumer == nil {
		return nil, componenterror.ErrNilNextConsumer
	}

	set.Logger.Info("Going to listen on endpoint for X-Ray segments",
		zap.String(udppoller.Transport, config.Endpoint))
	poller, err := udppoller.New(&udppoller.Config{
		ReceiverID:         config.ID(),
		Transport:          config.Transport,
		Endpoint:           config.Endpoint,
		NumOfPollerToStart: maxPollerCount,
	}, set)
	if err != nil {
		return nil, err
	}

	set.Logger.Info("Listening on endpoint for X-Ray segments",
		zap.String(udppoller.Transport, config.Endpoint))

	srv, err := proxy.NewServer(config.ProxyServer, set.Logger)
	if err != nil {
		return nil, err
	}

	return &xrayReceiver{
		instanceID: config.ID(),
		poller:     poller,
		server:     srv,
		settings:   set,
		consumer:   consumer,
		obsrecv: obsreport.NewReceiver(obsreport.ReceiverSettings{
			ReceiverID:             config.ID(),
			Transport:              udppoller.Transport,
			ReceiverCreateSettings: set,
		}),
	}, nil
}

func (x *xrayReceiver) Start(ctx context.Context, host component.Host) error {
	// TODO: Might want to pass `host` into read() below to report a fatal error
	x.poller.Start(ctx)
	go x.start()
	go x.server.ListenAndServe()
	x.settings.Logger.Info("X-Ray TCP proxy server started")
	return nil
}

func (x *xrayReceiver) Shutdown(ctx context.Context) error {
	var err error
	if pollerErr := x.poller.Close(); pollerErr != nil {
		err = pollerErr
	}

	if proxyErr := x.server.Shutdown(ctx); proxyErr != nil {
		if err == nil {
			err = proxyErr
		} else {
			err = fmt.Errorf("failed to close proxy: %s: failed to close poller: %s",
				proxyErr.Error(), err.Error())
		}
	}
	return err
}

func (x *xrayReceiver) start() {
	incomingSegments := x.poller.SegmentsChan()
	for seg := range incomingSegments {
		ctx := x.obsrecv.StartTracesOp(seg.Ctx)
		traces, totalSpanCount, err := translator.ToTraces(seg.Payload)
		if err != nil {
			x.settings.Logger.Warn("X-Ray segment to OT traces conversion failed", zap.Error(err))
			x.obsrecv.EndTracesOp(ctx, awsxray.TypeStr, totalSpanCount, err)
			continue
		}

		err = x.consumer.ConsumeTraces(ctx, *traces)
		if err != nil {
			x.settings.Logger.Warn("Trace consumer errored out", zap.Error(err))
			x.obsrecv.EndTracesOp(ctx, awsxray.TypeStr, totalSpanCount, err)
			continue
		}
		x.obsrecv.EndTracesOp(ctx, awsxray.TypeStr, totalSpanCount, nil)
	}
}
