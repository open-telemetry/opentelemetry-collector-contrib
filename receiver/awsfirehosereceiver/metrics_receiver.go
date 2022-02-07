// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awsfirehosereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver"

import (
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/unmarshaler"
)

var (
	errUnrecognizedEncoding = fmt.Errorf("unrecognized encoding")
	errMissingHost          = fmt.Errorf("nil host")
	errInvalidAccessKey     = fmt.Errorf("invalid firehose access key")
)

type firehoseMetricsReceiver struct {
	instanceID   config.ComponentID
	settings     component.ReceiverCreateSettings
	host         component.Host
	nextConsumer consumer.Metrics
	unmarshaler  unmarshaler.MetricsUnmarshaler

	config     *Config
	server     *http.Server
	shutdownWG sync.WaitGroup
}

type firehoseRequest struct {
	RequestId string `json:"requestId"`
	Timestamp int64  `json:"timestamp"`
	Records   []struct {
		// base64 encoded string
		Data string `json:"data"`
	} `json:"records"`
}

type firehoseResponse struct {
	RequestId    string `json:"requestId"`
	Timestamp    int64  `json:"timestamp"`
	ErrorMessage string `json:"errorMessage,omitempty"`
}

var _ component.Receiver = (*firehoseMetricsReceiver)(nil)
var _ http.Handler = (*firehoseMetricsReceiver)(nil)

func newMetricsReceiver(
	config *Config,
	set component.ReceiverCreateSettings,
	unmarshalers map[string]unmarshaler.MetricsUnmarshaler,
	nextConsumer consumer.Metrics,
) (component.MetricsReceiver, error) {
	if nextConsumer == nil {
		return nil, componenterror.ErrNilNextConsumer
	}

	configuredUnmarshaler := unmarshalers[config.Encoding]
	if configuredUnmarshaler == nil {
		return nil, errUnrecognizedEncoding
	}

	return &firehoseMetricsReceiver{
		instanceID:   config.ID(),
		settings:     set,
		nextConsumer: nextConsumer,
		unmarshaler:  configuredUnmarshaler,
		config:       config,
	}, nil
}

// Start spins up the receiver's HTTP server and makes the receiver start its processing.
func (fmr *firehoseMetricsReceiver) Start(_ context.Context, host component.Host) error {
	if host == nil {
		return errMissingHost
	}

	var err error
	fmr.host = host
	fmr.server, err = fmr.config.HTTPServerSettings.ToServer(host, fmr.settings.TelemetrySettings, fmr)
	if err != nil {
		return err
	}

	var listener net.Listener
	listener, err = fmr.config.HTTPServerSettings.ToListener()
	if err != nil {
		return err
	}
	fmr.shutdownWG.Add(1)
	go func() {
		defer fmr.shutdownWG.Done()

		if errHTTP := fmr.server.Serve(listener); !errors.Is(errHTTP, http.ErrServerClosed) && errHTTP != nil {
			host.ReportFatalError(errHTTP)
		}
	}()

	return nil
}

// Shutdown tells the receiver that should stop reception,
// giving it a chance to perform any necessary clean-up and shutting down
// its HTTP server.
func (fmr *firehoseMetricsReceiver) Shutdown(context.Context) error {
	err := fmr.server.Close()
	fmr.shutdownWG.Wait()
	return err
}

// ServeHTTP receives metrics as JSON, unmarshalls them,
// and sends them along to the nextConsumer.
func (fmr *firehoseMetricsReceiver) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	requestId := r.Header.Get("X-Amz-Firehose-Request-Id")
	fmr.settings.Logger.Debug("processing request", zap.String("requestId", requestId))

	if statusCode, err := fmr.validate(r); err != nil {
		fmr.sendResponse(w, requestId, statusCode, err.Error())
		return
	}

	body, err := fmr.getBody(r)
	if err != nil {
		fmr.sendResponse(w, requestId, http.StatusBadRequest, err.Error())
		return
	}

	var fr firehoseRequest
	if err = json.Unmarshal(body, &fr); err != nil {
		fmr.sendResponse(w, requestId, http.StatusBadRequest, err.Error())
		return
	}

	md := pdata.NewMetrics()
	for _, record := range fr.Records {
		if record.Data != "" {
			bytes, err := base64.StdEncoding.DecodeString(record.Data)
			if err != nil {
				fmr.sendResponse(w, requestId, http.StatusBadRequest, err.Error())
				return
			}

			rmd, err := fmr.unmarshaler.Unmarshal(bytes, fmr.settings.Logger)
			if err != nil {
				fmr.sendResponse(w, requestId, http.StatusBadRequest, err.Error())
				return
			}
			rmd.ResourceMetrics().MoveAndAppendTo(md.ResourceMetrics())
		}
	}

	err = fmr.nextConsumer.ConsumeMetrics(ctx, md)
	if err != nil {
		fmr.sendResponse(w, requestId, http.StatusInternalServerError, err.Error())
		return
	}

	fmr.sendResponse(w, requestId, http.StatusOK, "")
}

func (fmr *firehoseMetricsReceiver) validate(r *http.Request) (int, error) {
	if accessKey := r.Header.Get("X-Amz-Firehose-Access-Key"); accessKey != "" && accessKey != fmr.config.AccessKey {
		return http.StatusUnauthorized, errInvalidAccessKey
	}
	return http.StatusAccepted, nil
}

func (fmr *firehoseMetricsReceiver) getBody(r *http.Request) ([]byte, error) {
	reader := fmr.getReader(r.Body, r.Header.Get("Content-Encoding"))

	body, _ := ioutil.ReadAll(reader)
	if c, ok := reader.(io.Closer); ok {
		_ = c.Close()
	}
	_ = r.Body.Close()
	return body, nil
}

func (fmr *firehoseMetricsReceiver) getReader(r io.Reader, encoding string) io.Reader {
	if encoding == "gzip" {
		if gzr, err := gzip.NewReader(r); err == nil {
			return gzr
		}
	}
	return r
}

func (fmr *firehoseMetricsReceiver) getCommonAttributes(r *http.Request) (map[string]string, error) {
	var metadata map[string]map[string]string
	ca := r.Header.Get("X-Amz-Firehose-Common-Attributes")
	if ca != "" {
		if err := json.Unmarshal([]byte(ca), &metadata); err != nil {
			return nil, err
		}
	}
	return metadata["commonAttributes"], nil
}

func (fmr *firehoseMetricsReceiver) sendResponse(w http.ResponseWriter, requestId string, statusCode int, errorMessage string) {
	body := firehoseResponse{
		RequestId:    requestId,
		Timestamp:    time.Now().UnixMilli(),
		ErrorMessage: errorMessage,
	}
	payload, _ := json.Marshal(body)
	w.WriteHeader(statusCode)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
	if _, err := w.Write(payload); err != nil {
		fmr.settings.Logger.Error("failed to send response", zap.Error(err))
	}
}
