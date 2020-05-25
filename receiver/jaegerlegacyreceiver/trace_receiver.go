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

package jaegerlegacyreceiver

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/jaegertracing/jaeger/thrift-gen/jaeger"
	"github.com/uber/tchannel-go"
	"github.com/uber/tchannel-go/thrift"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/obsreport"
	jaegertranslator "go.opentelemetry.io/collector/translator/trace/jaeger"
	"go.uber.org/zap"
)

var (
	batchSubmitNotOkResponse = &jaeger.BatchSubmitResponse{}
	batchSubmitOkResponse    = &jaeger.BatchSubmitResponse{Ok: true}
)

// Configuration defines the behavior and the ports that
// the Jaeger Legacy receiver will use.
type Configuration struct {
	CollectorThriftPort int
}

// Receiver type is used to receive spans that were originally intended to be sent to Jaeger.
// This receiver is basically a Jaeger collector.
type jReceiver struct {
	// mu protects the fields of this type
	mu sync.Mutex

	nextConsumer consumer.TraceConsumerOld
	instanceName string

	startOnce sync.Once
	stopOnce  sync.Once

	config *Configuration

	tchanServer *jTchannelReceiver

	logger *zap.Logger
}

type jTchannelReceiver struct {
	nextConsumer consumer.TraceConsumerOld
	instanceName string

	tchannel *tchannel.Channel
}

const (
	// Legacy metrics receiver name tag values
	tchannelCollectorReceiverTagValue = "jaeger-tchannel-collector"

	collectorTChannelTransport = "collector_tchannel"

	thriftFormat = "thrift"
)

// New creates a TraceReceiver that receives traffic as a Jaeger collector, and
// also as a Jaeger agent.
func New(
	instanceName string,
	config *Configuration,
	nextConsumer consumer.TraceConsumerOld,
	logger *zap.Logger,
) (component.TraceReceiver, error) {
	return &jReceiver{
		config:       config,
		nextConsumer: nextConsumer,
		instanceName: instanceName,
		tchanServer: &jTchannelReceiver{
			nextConsumer: nextConsumer,
			instanceName: instanceName,
		},
		logger: logger,
	}, nil
}

var _ component.TraceReceiver = (*jReceiver)(nil)

func (jr *jReceiver) collectorThriftAddr() string {
	var port int
	if jr.config != nil {
		port = jr.config.CollectorThriftPort
	}
	return fmt.Sprintf(":%d", port)
}

func (jr *jReceiver) collectorThriftEnabled() bool {
	return jr.config != nil && jr.config.CollectorThriftPort > 0
}

func (jr *jReceiver) Start(_ context.Context, host component.Host) error {
	jr.mu.Lock()
	defer jr.mu.Unlock()

	var err = componenterror.ErrAlreadyStarted
	jr.startOnce.Do(func() {
		if err = jr.startCollector(host); err != nil && err != componenterror.ErrAlreadyStarted {
			jr.stopTraceReceptionLocked()
			return
		}

		err = nil
	})
	return err
}

func (jr *jReceiver) Shutdown(context.Context) error {
	jr.mu.Lock()
	defer jr.mu.Unlock()

	return jr.stopTraceReceptionLocked()
}

func (jr *jReceiver) stopTraceReceptionLocked() error {
	var err = componenterror.ErrAlreadyStopped
	jr.stopOnce.Do(func() {
		err = nil
		if jr.tchanServer.tchannel != nil {
			jr.tchanServer.tchannel.Close()
			jr.tchanServer.tchannel = nil
		}
	})

	return err
}

func consumeTraceData(
	ctx context.Context,
	batches []*jaeger.Batch,
	consumer consumer.TraceConsumerOld,
) ([]*jaeger.BatchSubmitResponse, int, error) {

	jbsr := make([]*jaeger.BatchSubmitResponse, 0, len(batches))
	var consumerError error
	numSpans := 0
	for _, batch := range batches {
		numSpans += len(batch.Spans)
		if consumerError != nil {
			jbsr = append(jbsr, batchSubmitNotOkResponse)
			continue
		}

		// TODO: function below never returns error, change the signature.
		td, _ := jaegertranslator.ThriftBatchToOCProto(batch)
		td.SourceFormat = "jaeger"
		consumerError = consumer.ConsumeTraceData(ctx, td)
		jsr := batchSubmitOkResponse
		if consumerError != nil {
			jsr = batchSubmitNotOkResponse
		}
		jbsr = append(jbsr, jsr)
	}

	return jbsr, numSpans, consumerError
}

func (jtr *jTchannelReceiver) SubmitBatches(thriftCtx thrift.Context, batches []*jaeger.Batch) ([]*jaeger.BatchSubmitResponse, error) {
	ctx := obsreport.ReceiverContext(
		thriftCtx, jtr.instanceName, collectorTChannelTransport, tchannelCollectorReceiverTagValue)
	ctx = obsreport.StartTraceDataReceiveOp(ctx, jtr.instanceName, collectorTChannelTransport)

	jbsr, numSpans, err := consumeTraceData(ctx, batches, jtr.nextConsumer)
	obsreport.EndTraceDataReceiveOp(ctx, thriftFormat, numSpans, err)

	return jbsr, err
}

func (jr *jReceiver) startCollector(host component.Host) error {
	if !jr.collectorThriftEnabled() {
		return nil
	}

	if jr.collectorThriftEnabled() {
		tch, terr := tchannel.NewChannel("jaeger-collector", new(tchannel.ChannelOptions))
		if terr != nil {
			return fmt.Errorf("failed to create NewTChannel: %v", terr)
		}

		server := thrift.NewServer(tch)
		server.Register(jaeger.NewTChanCollectorServer(jr.tchanServer))

		taddr := jr.collectorThriftAddr()
		tln, terr := net.Listen("tcp", taddr)
		if terr != nil {
			return fmt.Errorf("failed to bind to TChannel address %q: %v", taddr, terr)
		}
		tch.Serve(tln)
		jr.tchanServer.tchannel = tch
	}

	return nil
}
