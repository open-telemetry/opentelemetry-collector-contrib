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
	"go.opentelemetry.io/collector/config"
	"go.uber.org/zap"
)

const (
	headerFirehoseRequestId        = "X-Amz-Firehose-Request-Id"
	headerFirehoseAccessKey        = "X-Amz-Firehose-Access-Key"
	headerFirehoseCommonAttributes = "X-Amz-Firehose-Common-Attributes"
)

var (
	errUnrecognizedEncoding = fmt.Errorf("unrecognized encoding")
	errMissingHost          = fmt.Errorf("nil host")
	errInvalidAccessKey     = fmt.Errorf("invalid firehose access key")
)

// The firehoseConsumer is responsible for using the unmarshaler and the consumer.
type firehoseConsumer interface {
	// consume unmarshalls and consumes the records
	consume(ctx context.Context, records [][]byte) (int, error)
}

type firehoseReceiver struct {
	instanceID config.ComponentID
	settings   component.ReceiverCreateSettings
	host       component.Host

	config     *Config
	server     *http.Server
	shutdownWG sync.WaitGroup
	consumer   firehoseConsumer
}

// firehoseRequest is based on https://docs.aws.amazon.com/firehose/latest/dev/httpdeliveryrequestresponse.html
type firehoseRequest struct {
	RequestId string           `json:"requestId"`
	Timestamp int64            `json:"timestamp"`
	Records   []firehoseRecord `json:"records"`
}

type firehoseRecord struct {
	Data string `json:"data"` // base64 encoded string
}

type firehoseResponse struct {
	RequestId    string `json:"requestId"`
	Timestamp    int64  `json:"timestamp"`
	ErrorMessage string `json:"errorMessage,omitempty"`
}

type firehoseCommonAttributes struct {
	CommonAttributes map[string]string `json:"commonAttributes"`
}

var _ component.Receiver = (*firehoseReceiver)(nil)
var _ http.Handler = (*firehoseReceiver)(nil)

// Start spins up the receiver's HTTP server and makes the receiver start its processing.
func (fmr *firehoseReceiver) Start(_ context.Context, host component.Host) error {
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
func (fmr *firehoseReceiver) Shutdown(context.Context) error {
	err := fmr.server.Close()
	fmr.shutdownWG.Wait()
	return err
}

// ServeHTTP receives metrics as JSON, unmarshalls them,
// and sends them along to the nextConsumer.
func (fmr *firehoseReceiver) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	requestId := r.Header.Get(headerFirehoseRequestId)
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

	records := make([][]byte, 0, len(fr.Records))
	for _, record := range fr.Records {
		if record.Data != "" {
			decoded, err := base64.StdEncoding.DecodeString(record.Data)
			if err != nil {
				fmr.sendResponse(w, requestId, http.StatusBadRequest, err.Error())
				return
			}

			records = append(records, decoded)
		}
	}

	statusCode, err := fmr.consumer.consume(ctx, records)
	if err != nil {
		fmr.sendResponse(w, requestId, statusCode, err.Error())
	}

	fmr.sendResponse(w, requestId, http.StatusOK, "")
}

func (fmr *firehoseReceiver) validate(r *http.Request) (int, error) {
	if accessKey := r.Header.Get(headerFirehoseAccessKey); accessKey != "" && accessKey != fmr.config.AccessKey {
		return http.StatusUnauthorized, errInvalidAccessKey
	}
	return http.StatusAccepted, nil
}

func (fmr *firehoseReceiver) getBody(r *http.Request) ([]byte, error) {
	reader := fmr.getReader(r.Body, r.Header.Get("Content-Encoding"))

	body, _ := ioutil.ReadAll(reader)
	if c, ok := reader.(io.Closer); ok {
		_ = c.Close()
	}
	_ = r.Body.Close()
	return body, nil
}

func (fmr *firehoseReceiver) getReader(r io.Reader, encoding string) io.Reader {
	if encoding == "gzip" {
		if gzr, err := gzip.NewReader(r); err == nil {
			return gzr
		}
	}
	return r
}

func (fmr *firehoseReceiver) getCommonAttributes(r *http.Request) (map[string]string, error) {
	attributes := make(map[string]string)
	if commonAttributes := r.Header.Get(headerFirehoseCommonAttributes); commonAttributes != "" {
		var fca firehoseCommonAttributes
		if err := json.Unmarshal([]byte(commonAttributes), &fca); err != nil {
			return nil, err
		}
		attributes = fca.CommonAttributes
	}
	return attributes, nil
}

func (fmr *firehoseReceiver) sendResponse(w http.ResponseWriter, requestId string, statusCode int, errorMessage string) {
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
