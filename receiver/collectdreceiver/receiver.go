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

package collectdreceiver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/translator/internaldata"
	"go.uber.org/zap"
)

var (
	errNilNextConsumer = errors.New("nil nextConsumer")
	errAlreadyStarted  = errors.New("already started")
	errAlreadyStopped  = errors.New("already stopped")
)

var _ component.MetricsReceiver = (*collectdReceiver)(nil)

// collectdReceiver implements the component.MetricsReceiver for CollectD protocol.
type collectdReceiver struct {
	sync.Mutex
	logger             *zap.Logger
	addr               string
	server             *http.Server
	defaultAttrsPrefix string
	nextConsumer       consumer.MetricsConsumer

	startOnce sync.Once
	stopOnce  sync.Once
}

// newCollectdReceiver creates the CollectD receiver with the given parameters.
func newCollectdReceiver(
	logger *zap.Logger,
	addr string,
	timeout time.Duration,
	defaultAttrsPrefix string,
	nextConsumer consumer.MetricsConsumer) (component.MetricsReceiver, error) {
	if nextConsumer == nil {
		return nil, errNilNextConsumer
	}

	r := &collectdReceiver{
		logger:             logger,
		addr:               addr,
		nextConsumer:       nextConsumer,
		defaultAttrsPrefix: defaultAttrsPrefix,
	}
	r.server = &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  timeout,
		WriteTimeout: timeout,
	}
	return r, nil
}

// StartMetricsReception starts an HTTP server that can process CollectD JSON requests.
func (cdr *collectdReceiver) Start(_ context.Context, host component.Host) error {
	cdr.Lock()
	defer cdr.Unlock()

	err := errAlreadyStarted
	cdr.startOnce.Do(func() {
		err = nil
		go func() {
			err = cdr.server.ListenAndServe()
			if err != nil {
				host.ReportFatalError(fmt.Errorf("error starting collectd receiver: %v", err))
			}
		}()
	})

	return err
}

// StopMetricsReception stops the CollectD receiver.
func (cdr *collectdReceiver) Shutdown(context.Context) error {
	cdr.Lock()
	defer cdr.Unlock()

	var err = errAlreadyStopped
	cdr.stopOnce.Do(func() {
		err = cdr.server.Shutdown(context.Background())
	})
	return err
}

// ServeHTTP acts as the default and only HTTP handler for the CollectD receiver.
func (cdr *collectdReceiver) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	recordRequestReceived()

	if r.Method != "POST" {
		recordRequestErrors()
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		recordRequestErrors()
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var records []collectDRecord
	err = json.Unmarshal(body, &records)
	if err != nil {
		cdr.handleHTTPErr(w, err, "unable to decode json")
		return
	}

	defaultAttrs := cdr.defaultAttributes(r)

	md := consumerdata.MetricsData{}
	ctx := context.Background()
	for _, record := range records {
		md.Metrics, err = record.appendToMetrics(md.Metrics, defaultAttrs)
		if err != nil {
			cdr.handleHTTPErr(w, err, "unable to process metrics")
			return
		}
	}

	err = cdr.nextConsumer.ConsumeMetrics(ctx, internaldata.OCToMetrics(md))
	if err != nil {
		cdr.handleHTTPErr(w, err, "unable to process metrics")
		return
	}
	w.Write([]byte("OK"))
}

func (cdr *collectdReceiver) defaultAttributes(req *http.Request) map[string]string {
	if cdr.defaultAttrsPrefix == "" {
		return nil
	}
	params := req.URL.Query()
	attrs := make(map[string]string)
	for key := range params {
		if strings.HasPrefix(key, cdr.defaultAttrsPrefix) {
			value := params.Get(key)
			if len(value) == 0 {
				recordDefaultBlankAttrs()
				continue
			}
			key = key[len(cdr.defaultAttrsPrefix):]
			attrs[key] = value
		}
	}
	return attrs
}

func (cdr *collectdReceiver) handleHTTPErr(w http.ResponseWriter, err error, msg string) {
	recordRequestErrors()
	w.WriteHeader(http.StatusBadRequest)
	cdr.logger.Error(msg, zap.Error(err))
	_, err = w.Write([]byte(msg))
	if err != nil {
		cdr.logger.Error("error writing to response writer", zap.Error(err))
	}
}
