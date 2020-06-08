package statsdreceiver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"
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
	logger             *zap.Logger
	addr               string
	server             *http.Server
	defaultAttrsPrefix string
	nextConsumer       consumer.MetricsConsumerOld

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

	r := &statsdReceiver{
		logger:       logger,
		addr:         addr,
		nextConsumer: nextConsumer,
	}
	r.server = &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  timeout,
		WriteTimeout: timeout,
	}
	return r, nil
}

// StartMetricsReception starts an HTTP server that can process StatsD JSON requests.
func (ddr *statsdReceiver) Start(_ context.Context, host component.Host) error {
	ddr.Lock()
	defer ddr.Unlock()

	err := errAlreadyStarted
	ddr.startOnce.Do(func() {
		err = nil
		go func() {
			err = ddr.server.ListenAndServe()
			if err != nil {
				host.ReportFatalError(fmt.Errorf("error starting statsd receiver: %v", err))
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
		err = ddr.server.Shutdown(context.Background())
	})
	return err
}

// ServeHTTP acts as the default and only HTTP handler for the StatsD receiver.
func (ddr *statsdReceiver) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.Write([]byte("OK"))
}

func (ddr *statsdReceiver) handleHTTPErr(w http.ResponseWriter, err error, msg string) {
	w.WriteHeader(http.StatusBadRequest)
	ddr.logger.Error(msg, zap.Error(err))
	_, err = w.Write([]byte(msg))
	if err != nil {
		ddr.logger.Error("error writing to response writer", zap.Error(err))
	}
}
