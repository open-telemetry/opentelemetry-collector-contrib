// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package netflowreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/netflowreceiver"

import (
	"errors"
	"fmt"
	"net"

	"github.com/netsampler/goflow2/v2/decoders/netflow"

	protoproducer "github.com/netsampler/goflow2/v2/producer/proto"
	"github.com/netsampler/goflow2/v2/utils"
	"github.com/netsampler/goflow2/v2/utils/debug"

	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"
)

type Listener struct {
	config      Config
	logger      *zap.Logger
	recv        *utils.UDPReceiver
	logConsumer consumer.Logs
}

func (l *Listener) Dropped(msg utils.Message) {
	l.logger.Warn("Dropped netflow message", zap.Any("msg", msg))
}

func newListener(config Config, logger *zap.Logger, logConsumer consumer.Logs) *Listener {
	return &Listener{config: config, logger: logger, logConsumer: logConsumer}
}

func (l *Listener) Start() error {
	l.logger.Info("Creating the netflow UDP listener", zap.Any("config", l.config))
	cfg := &utils.UDPReceiverConfig{
		Sockets:          l.config.Sockets,
		Workers:          l.config.Workers,
		QueueSize:        l.config.QueueSize,
		Blocking:         false,
		ReceiverCallback: l,
	}
	recv, err := utils.NewUDPReceiver(cfg)
	if err != nil {
		return err
	}
	l.recv = recv

	decodeFunc, err := l.buildDecodeFunc()
	if err != nil {
		return err
	}

	l.logger.Info("Start listening for NetFlow", zap.Any("config", l.config))
	if err := l.recv.Start(l.config.Hostname, l.config.Port, decodeFunc); err != nil {
		return err
	}

	go l.handleErrors()

	return nil
}

// handleErrors handles errors from the listener
func (l *Listener) handleErrors() {
	for err := range l.recv.Errors() {
		if errors.Is(err, net.ErrClosed) {
			l.logger.Info("receiver closed")
			continue
		} else if !errors.Is(err, netflow.ErrorTemplateNotFound) && !errors.Is(err, debug.PanicError) {
			l.logger.Error("receiver error", zap.Error(err))
			continue
		} else if errors.Is(err, netflow.ErrorTemplateNotFound) {
			l.logger.Warn("template was not found for this message")
			continue
		} else if errors.Is(err, debug.PanicError) {
			var pErrMsg *debug.PanicErrorMessage
			if errors.As(err, &pErrMsg) {
				l.logger.Error("panic error", zap.String("panic", pErrMsg.Inner))
				l.logger.Error("receiver stacktrace", zap.String("stack", string(pErrMsg.Stacktrace)))
				l.logger.Error("receiver msg", zap.Any("error", pErrMsg.Msg))
			}
			l.logger.Error("receiver panic", zap.Error(err))

			continue
		}
	}
}

// buildDecodeFunc creates a decode function based on the scheme
func (l *Listener) buildDecodeFunc() (utils.DecoderFunc, error) {

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
	otelLogsProducer := newOtelLogsProducer(protoProducer, l.logConsumer)

	cfgPipe := &utils.PipeConfig{
		Producer: otelLogsProducer,
		// Format:   &format.Format{
		// 	FormatDriver: &format.JSONFormatDriver{},
		// },
	}

	var decodeFunc utils.DecoderFunc
	var p utils.FlowPipe
	if l.config.Scheme == "sflow" {
		p = utils.NewSFlowPipe(cfgPipe)
	} else if l.config.Scheme == "netflow" {
		p = utils.NewNetFlowPipe(cfgPipe)
	} else if l.config.Scheme == "flow" {
		p = utils.NewFlowPipe(cfgPipe)
	} else {
		return nil, fmt.Errorf("scheme does not exist: %s", l.config.Scheme)
	}

	decodeFunc = p.DecodeFlow

	// We wrap panics while decoding the message to habndle them later
	decodeFunc = debug.PanicDecoderWrapper(decodeFunc)

	return decodeFunc, nil
}

func (l *Listener) Shutdown() error {
	if l.recv != nil {
		return l.recv.Stop()
	}
	return nil
}
