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
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

var _ utils.ReceiverCallback = (*dropHandler)(nil)

type dropHandler struct {
	logger *zap.Logger
}

func (d dropHandler) Dropped(msg utils.Message) {
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

func (nr *netflowReceiver) Start(_ context.Context, _ component.Host) error {
	// The function that will decode packets
	decodeFunc, err := nr.buildDecodeFunc()
	if err != nil {
		return err
	}

	nr.logger.Info("Starting UDP listener", zap.String("scheme", nr.config.Scheme), zap.Int("port", nr.config.Port))
	if err := nr.udpReceiver.Start(nr.config.Hostname, nr.config.Port, decodeFunc); err != nil {
		return err
	}

	// This runs until the receiver is stoppped, consuming from an error channel
	go nr.handleErrors()

	return nil
}

func (nr *netflowReceiver) Shutdown(context.Context) error {
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
	otelLogsProducer := newOtelLogsProducer(protoProducer, nr.logConsumer, nr.logger, nr.config.SendRaw)

	cfgPipe := &utils.PipeConfig{
		Producer: otelLogsProducer,
	}

	var p utils.FlowPipe
	switch nr.config.Scheme {
	case "sflow":
		p = utils.NewSFlowPipe(cfgPipe)
	case "netflow":
		p = utils.NewNetFlowPipe(cfgPipe)
	default:
		return nil, fmt.Errorf("scheme does not exist: %s", nr.config.Scheme)
	}
	return p.DecodeFlow, nil
}

// handleErrors handles errors from the listener
// We don't want the receiver to stop if there is an error processing a packet
func (nr *netflowReceiver) handleErrors() {
	for err := range nr.udpReceiver.Errors() {
		switch {
		case errors.Is(err, net.ErrClosed):
			nr.logger.Info("UDP receiver closed, exiting error handler")
			return

		case !errors.Is(err, netflow.ErrorTemplateNotFound):
			nr.logger.Error("received a generic error while processing a flow message via GoFlow2 for the netflow receiver", zap.Error(err))
			continue

		case errors.Is(err, netflow.ErrorTemplateNotFound):
			nr.logger.Warn("we could not find a template for a flow message, this error is expected from time to time until the device sends a template", zap.Error(err))
			continue

		default:
			nr.logger.Error("unexpected error processing the message", zap.Error(err))
			continue
		}
	}
}
