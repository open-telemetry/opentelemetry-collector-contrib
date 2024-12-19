// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package netflowreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/netflowreceiver"

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/netsampler/goflow2/v2/decoders/netflow"
	protoproducer "github.com/netsampler/goflow2/v2/producer/proto"
	"github.com/netsampler/goflow2/v2/utils"
	"github.com/netsampler/goflow2/v2/utils/debug"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

type dropHandler struct {
	logger *zap.Logger
}

func (d *dropHandler) Dropped(msg utils.Message) {
	d.logger.Warn("Dropped netflow message", zap.Any("msg", msg))
}

type netflowReceiver struct {
	config      Config
	logger      *zap.Logger
	udpReceiver *utils.UDPReceiver
	logConsumer consumer.Logs
}

func newNetflowLogsReceiver(params receiver.Settings, cfg Config, consumer consumer.Logs) (receiver.Logs, error) {
	// UDP receiver configuration
	udpCfg := &utils.UDPReceiverConfig{
		Sockets:   cfg.Sockets,
		Workers:   cfg.Workers,
		QueueSize: cfg.QueueSize,
		Blocking:  false,
		ReceiverCallback: &dropHandler{
			logger: params.Logger,
		},
	}
	udpReceiver, err := utils.NewUDPReceiver(udpCfg)
	if err != nil {
		return nil, err
	}

	nr := &netflowReceiver{
		logger:      params.Logger,
		config:      cfg,
		logConsumer: consumer,
		udpReceiver: udpReceiver,
	}

	return nr, nil
}

func (nr *netflowReceiver) Start(ctx context.Context, host component.Host) error {
	nr.logger.Info("NetFlow receiver is starting...")

	// The function that will decode packets
	decodeFunc, err := nr.buildDecodeFunc()
	if err != nil {
		return err
	}

	nr.logger.Info("Start listening for NetFlow on UDP", zap.Any("config", nr.config))
	if err := nr.udpReceiver.Start(nr.config.Hostname, nr.config.Port, decodeFunc); err != nil {
		return err
	}

	// This runs until the receiver is stoppped, consuming from an error channel
	go nr.handleErrors()

	return nil
}

func (nr *netflowReceiver) Shutdown(context.Context) error {
	nr.logger.Info("NetFlow receiver is shutting down...")
	if nr.udpReceiver == nil {
		return nil
	}
	err := nr.udpReceiver.Stop()
	if err != nil {
		nr.logger.Warn("Error stopping UDP receiver", zap.Error(err))
	}
	return nil
}

// buildDecodeFunc creates a decode function based on the scheme
// This is the fuction that will be invoked for every netflow packet received
// The function depends on the type of schema (netflow, sflow, flow)
func (nr *netflowReceiver) buildDecodeFunc() (utils.DecoderFunc, error) {
	// Eventually this can be used to configure mappings
	cfgProducer := &protoproducer.ProducerConfig{}
	cfgm, err := cfgProducer.Compile() // converts configuration into a format that can be used by a protobuf producer
	if err != nil {
		return nil, err
	}
	// We use a goflow2 proto producer to produce messages using protobuf format
	protoProducer, err := protoproducer.CreateProtoProducer(cfgm, protoproducer.CreateSamplingSystem)
	if err != nil {
		return nil, err
	}

	// the otel log producer converts those messages into OpenTelemetry logs
	// it is a wrapper around the protobuf producer
	otelLogsProducer := newOtelLogsProducer(protoProducer, nr.logConsumer)

	cfgPipe := &utils.PipeConfig{
		Producer: otelLogsProducer,
	}

	var decodeFunc utils.DecoderFunc
	var p utils.FlowPipe
	switch nr.config.Scheme {
	case "sflow":
		p = utils.NewSFlowPipe(cfgPipe)
	case "netflow":
		p = utils.NewNetFlowPipe(cfgPipe)
	case "flow":
		p = utils.NewFlowPipe(cfgPipe)
	default:
		return nil, fmt.Errorf("scheme does not exist: %s", nr.config.Scheme)
	}

	decodeFunc = p.DecodeFlow

	// We wrap panics while decoding the message to habndle them later
	decodeFunc = debug.PanicDecoderWrapper(decodeFunc)

	return decodeFunc, nil
}

// handleErrors handles errors from the listener
// These come from the panic decoder wrapper around the decode function
// We don't want the receiver to stop if there is an error processing a packet
func (nr *netflowReceiver) handleErrors() {
	for err := range nr.udpReceiver.Errors() {
		switch {
		case errors.Is(err, net.ErrClosed):
			nr.logger.Info("UDP receiver closed, exiting error handler")
			return

		case !errors.Is(err, netflow.ErrorTemplateNotFound) && !errors.Is(err, debug.PanicError):
			nr.logger.Error("receiver error", zap.Error(err))
			continue

		case errors.Is(err, netflow.ErrorTemplateNotFound):
			nr.logger.Warn("template was not found for this message")
			continue

		case errors.Is(err, debug.PanicError):
			var pErrMsg *debug.PanicErrorMessage
			if errors.As(err, &pErrMsg) {
				nr.logger.Error("panic error", zap.String("panic", pErrMsg.Inner))
				nr.logger.Error("receiver stacktrace", zap.String("stack", string(pErrMsg.Stacktrace)))
				nr.logger.Error("receiver msg", zap.Any("error", pErrMsg.Msg))
			}
			nr.logger.Error("receiver panic", zap.Error(err))
			continue
		}
	}
}
