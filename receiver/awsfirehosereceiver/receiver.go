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
	"net"
	"net/http"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.uber.org/zap"
)

const (
	headerFirehoseRequestID        = "X-Amz-Firehose-Request-Id"
	headerFirehoseAccessKey        = "X-Amz-Firehose-Access-Key"
	headerFirehoseCommonAttributes = "X-Amz-Firehose-Common-Attributes"
	headerContentEncoding          = "Content-Encoding"
	headerContentType              = "Content-Type"
	headerContentLength            = "Content-Length"
)

var (
	errUnrecognizedEncoding     = errors.New("unrecognized encoding")
	errMissingHost              = errors.New("nil host")
	errInvalidAccessKey         = errors.New("invalid firehose access key")
	errInHeaderMissingRequestID = errors.New("missing request id in header")
	errInBodyMissingRequestID   = errors.New("missing request id in body")
	errInBodyDiffRequestID      = errors.New("different request id in body")
)

// The firehoseConsumer is responsible for using the unmarshaler and the consumer.
type firehoseConsumer interface {
	// Consume unmarshalls and consumes the records.
	Consume(ctx context.Context, records [][]byte, commonAttributes map[string]string) (int, error)
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
	RequestID string           `json:"requestId"`
	Timestamp int64            `json:"timestamp"`
	Records   []firehoseRecord `json:"records"`
}

type firehoseRecord struct {
	Data string `json:"data"` // base64 encoded string
}

type firehoseResponse struct {
	RequestID    string `json:"requestId"`
	Timestamp    int64  `json:"timestamp"`
	ErrorMessage string `json:"errorMessage,omitempty"`
}

type firehoseCommonAttributes struct {
	CommonAttributes map[string]string `json:"commonAttributes"`
}

var _ component.Receiver = (*firehoseReceiver)(nil)
var _ http.Handler = (*firehoseReceiver)(nil)

// Start spins up the receiver's HTTP server and makes the receiver start
// its processing.
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
// giving it a chance to perform any necessary clean-up and
// shutting down its HTTP server.
func (fmr *firehoseReceiver) Shutdown(context.Context) error {
	err := fmr.server.Close()
	fmr.shutdownWG.Wait()
	return err
}

// ServeHTTP receives Firehose requests, unmarshalls them, and sends them along to the firehoseConsumer.
func (fmr *firehoseReceiver) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	requestID := r.Header.Get(headerFirehoseRequestID)
	if requestID == "" {
		fmr.sendResponse(w, requestID, http.StatusBadRequest, errInHeaderMissingRequestID)
		return
	}
	fmr.settings.Logger.Debug("processing request", zap.String("requestID", requestID))

	if statusCode, err := fmr.validate(r); err != nil {
		fmr.sendResponse(w, requestID, statusCode, err)
		return
	}

	body := fmr.getBody(r)

	var fr firehoseRequest
	if err := json.Unmarshal(body, &fr); err != nil {
		fmr.sendResponse(w, requestID, http.StatusBadRequest, err)
		return
	}

	if fr.RequestID == "" {
		fmr.sendResponse(w, requestID, http.StatusBadRequest, errInBodyMissingRequestID)
		return
	} else if fr.RequestID != requestID {
		fmr.sendResponse(w, requestID, http.StatusBadRequest, errInBodyDiffRequestID)
		return
	}

	records := make([][]byte, 0, len(fr.Records))
	for _, record := range fr.Records {
		if record.Data != "" {
			decoded, err := base64.StdEncoding.DecodeString(record.Data)
			if err != nil {
				fmr.sendResponse(w, requestID, http.StatusBadRequest, err)
				return
			}

			records = append(records, decoded)
		}
	}

	commonAttributes, _ := fmr.getCommonAttributes(r)

	statusCode, err := fmr.consumer.Consume(ctx, records, commonAttributes)
	if err != nil {
		fmr.sendResponse(w, requestID, statusCode, err)
		return
	}

	fmr.sendResponse(w, requestID, http.StatusOK, nil)
}

// validate checks the Firehose access key in the header against
// the one passed into the Config
func (fmr *firehoseReceiver) validate(r *http.Request) (int, error) {
	if accessKey := r.Header.Get(headerFirehoseAccessKey); accessKey != "" && accessKey != fmr.config.AccessKey {
		return http.StatusUnauthorized, errInvalidAccessKey
	}
	return http.StatusAccepted, nil
}

// getBody reads the body from the request as a slice of bytes.
func (fmr *firehoseReceiver) getBody(r *http.Request) []byte {
	reader := fmr.getReader(r.Body, r.Header.Get(headerContentEncoding))

	body, _ := io.ReadAll(reader)
	if c, ok := reader.(io.Closer); ok {
		_ = c.Close()
	}
	_ = r.Body.Close()
	return body
}

// getReader uses the encoding to wrap the reader if matched otherwise returns
// the original reader.
func (fmr *firehoseReceiver) getReader(r io.Reader, encoding string) io.Reader {
	if encoding == "gzip" {
		if gzr, err := gzip.NewReader(r); err == nil {
			return gzr
		}
	}
	return r
}

// getCommonAttributes unmarshalls the common attributes from the request header
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

// sendResponse writes a response to Firehose in the expected format.
func (fmr *firehoseReceiver) sendResponse(w http.ResponseWriter, requestID string, statusCode int, err error) {
	var errorMessage string
	if err != nil {
		errorMessage = err.Error()
	}
	body := firehoseResponse{
		RequestID:    requestID,
		Timestamp:    time.Now().UnixMilli(),
		ErrorMessage: errorMessage,
	}
	payload, _ := json.Marshal(body)
	w.WriteHeader(statusCode)
	w.Header().Set(headerContentType, "application/json")
	w.Header().Set(headerContentLength, fmt.Sprintf("%d", len(payload)))
	if _, err = w.Write(payload); err != nil {
		fmr.settings.Logger.Error("failed to send response", zap.Error(err))
	}
}
