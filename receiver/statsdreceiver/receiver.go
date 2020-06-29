package statsdreceiver

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/protocol"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/transport"
)

var (
	errNilNextConsumer = errors.New("nil nextConsumer")
	errAlreadyStarted  = errors.New("already started")
	errAlreadyStopped  = errors.New("already stopped")
)

var _ component.MetricsReceiver = (*statsdReceiver)(nil)

// statsdReceiver implements the component.MetricsReceiver for StatsD protocol.
type statsdReceiver struct {
	sync.Mutex
	logger *zap.Logger
	addr   string
	// config *Config

	server       transport.Server
	parser       protocol.Parser
	nextConsumer consumer.MetricsConsumerOld

	startOnce sync.Once
	stopOnce  sync.Once
}

// New creates the StatsD receiver with the given parameters.
func New(
	logger *zap.Logger,
	addr string,
	timeout time.Duration,
	nextConsumer consumer.MetricsConsumerOld) (component.MetricsReceiver, error) {
	if nextConsumer == nil {
		return nil, errNilNextConsumer
	}

	if addr == "" {
		addr = "localhost:8125"
	}

	server, err := transport.NewUDPServer(addr)
	if err != nil {
		return nil, err
	}

	r := &statsdReceiver{
		logger:       logger,
		addr:         addr,
		parser:       &protocol.DogStatsDParser{},
		nextConsumer: nextConsumer,
		server:       server,
	}
	return r, nil
}

// StartMetricsReception starts an HTTP server that can process StatsD JSON requests.
func (ddr *statsdReceiver) Start(_ context.Context, host component.Host) error {
	ddr.Lock()
	defer ddr.Unlock()

	err := componenterror.ErrAlreadyStarted
	ddr.startOnce.Do(func() {
		err = nil
		go func() {
			err = ddr.server.ListenAndServe(ddr.parser, ddr.nextConsumer)
			if err != nil {
				host.ReportFatalError(err)
			}
		}()
	})

	return err
}

// StopMetricsReception stops the StatsD receiver.
func (ddr *statsdReceiver) Shutdown(context.Context) error {
	ddr.Lock()
	defer ddr.Unlock()

	var err = errAlreadyStopped
	ddr.stopOnce.Do(func() {
		err = ddr.server.Close()
	})
	return err
}
