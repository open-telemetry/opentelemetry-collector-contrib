// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsfirehosereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver"

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	jsoniter "github.com/json-iterator/go"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

const (
	headerFirehoseRequestID        = "X-Amz-Firehose-Request-Id"
	headerFirehoseAccessKey        = "X-Amz-Firehose-Access-Key"
	headerFirehoseCommonAttributes = "X-Amz-Firehose-Common-Attributes"
	headerContentType              = "Content-Type"
	headerContentLength            = "Content-Length"
)

var (
	errInvalidAccessKey         = errors.New("invalid firehose access key")
	errInHeaderMissingRequestID = errors.New("missing request id in header")
	errInBodyMissingRequestID   = errors.New("missing request id in body")
	errInBodyDiffRequestID      = errors.New("different request id in body")
)

// The firehoseConsumer is responsible for using the unmarshaler and the consumer.
type firehoseConsumer interface {
	Start(context.Context, component.Host) error

	// Consume unmarshals and consumes the records returned by f.
	Consume(ctx context.Context, f nextRecordFunc, commonAttributes map[string]string) (int, error)
}

// nextRecordFunc is a function provided to consumers for obtaining the
// next record to consume. The function returns (nil, io.EOF) when there
// are no more records.
type nextRecordFunc func() ([]byte, error)

// firehoseReceiver
type firehoseReceiver struct {
	// settings is the base receiver settings.
	settings receiver.Settings
	// config is the configuration for the receiver.
	config *Config
	// server is the HTTP/HTTPS server set up to listen
	// for requests.
	server *http.Server
	// shutdownWG is the WaitGroup that is used to wait until
	// the server shutdown has completed.
	shutdownWG sync.WaitGroup
	// consumer is the firehoseConsumer to use to process/send
	// the records in each request.
	consumer firehoseConsumer
}

// The firehoseRequest is the format of the received request body.
type firehoseRequest struct {
	// RequestID is a GUID that should be the same value as
	// the one in the header.
	RequestID string `json:"requestId"`
	// Timestamp is the milliseconds since epoch for when the
	// request was generated.
	Timestamp int64 `json:"timestamp"`
	// Records contains the data.
	Records []firehoseRecord `json:"records"`
}

// The firehoseRecord is an individual record within the firehoseRequest.
type firehoseRecord struct {
	// Data is a base64 encoded string. Can be empty.
	Data string `json:"data"`
}

// The firehoseResponse is the expected body for the response back to
// the delivery stream.
type firehoseResponse struct {
	// RequestID is the same GUID that was received in
	// the request.
	RequestID string `json:"requestId"`
	// Timestamp is the milliseconds since epoch for when the
	// request finished being processed.
	Timestamp int64 `json:"timestamp"`
	// ErrorMessage is the error to report. Empty if request
	// was successfully processed.
	ErrorMessage string `json:"errorMessage,omitempty"`
}

// The firehoseCommonAttributes is the format for the common attributes
// found in the header of requests.
type firehoseCommonAttributes struct {
	// CommonAttributes can be set when creating the delivery stream.
	// These will be passed to the firehoseConsumer, which should
	// attach the attributes.
	CommonAttributes map[string]string `json:"commonAttributes"`
}

var (
	_ receiver.Metrics = (*firehoseReceiver)(nil)
	_ http.Handler     = (*firehoseReceiver)(nil)
)

// Start spins up the receiver's HTTP server and makes the receiver start
// its processing.
func (fmr *firehoseReceiver) Start(ctx context.Context, host component.Host) error {
	if err := fmr.consumer.Start(ctx, host); err != nil {
		return fmt.Errorf("failed to start consumer: %w", err)
	}

	var err error
	fmr.server, err = fmr.config.ToServer(ctx, host.GetExtensions(), fmr.settings.TelemetrySettings, fmr)
	if err != nil {
		return fmt.Errorf("failed to initialize HTTP server: %w", err)
	}

	var listener net.Listener
	listener, err = fmr.config.ToListener(ctx)
	if err != nil {
		return fmt.Errorf("failed to start listening for HTTP requests: %w", err)
	}
	fmr.shutdownWG.Go(func() {
		if errHTTP := fmr.server.Serve(listener); errHTTP != nil && !errors.Is(errHTTP, http.ErrServerClosed) {
			componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(errHTTP))
		}
	})

	return nil
}

// Shutdown tells the receiver that should stop reception,
// giving it a chance to perform any necessary clean-up and
// shutting down its HTTP server.
func (fmr *firehoseReceiver) Shutdown(context.Context) error {
	if fmr.server == nil {
		return nil
	}
	err := fmr.server.Close()
	fmr.shutdownWG.Wait()
	return err
}

// ServeHTTP receives Firehose requests, unmarshalls them, and sends them along to the firehoseConsumer,
// which is responsible for unmarshalling the records and sending them to the next consumer.
func (fmr *firehoseReceiver) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	requestID := r.Header.Get(headerFirehoseRequestID)
	if requestID == "" {
		fmr.settings.Logger.Error(
			"Invalid Firehose request",
			zap.Error(errInHeaderMissingRequestID),
		)
		fmr.sendResponse(w, requestID, http.StatusBadRequest, errInHeaderMissingRequestID)
		return
	}
	fmr.settings.Logger.Debug("Processing Firehose request", zap.String("RequestID", requestID))

	if statusCode, err := fmr.validate(r); err != nil {
		fmr.settings.Logger.Error(
			"Invalid Firehose request",
			zap.Error(err),
		)
		fmr.sendResponse(w, requestID, statusCode, err)
		return
	}

	var fr firehoseRequest
	if err := jsoniter.ConfigFastest.NewDecoder(r.Body).Decode(&fr); err != nil {
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

	commonAttributes, err := fmr.getCommonAttributes(r)
	if err != nil {
		fmr.settings.Logger.Error(
			"Unable to get common attributes from request header. Will not attach attributes.",
			zap.Error(err),
		)
	}

	var recordIndex int
	var recordBuf []byte
	nextRecord := func() ([]byte, error) {
		if recordIndex == len(fr.Records) {
			return nil, io.EOF
		}
		record := fr.Records[recordIndex]
		recordIndex++

		var decodeErr error
		recordBuf, decodeErr = base64.StdEncoding.AppendDecode(recordBuf[:0], []byte(record.Data))
		if decodeErr != nil {
			return nil, fmt.Errorf("unable to base64 decode the record at index %d: %w", recordIndex-1, decodeErr)
		}
		return recordBuf, nil
	}

	statusCode, err := fmr.consumer.Consume(ctx, nextRecord, commonAttributes)
	if err != nil {
		fmr.settings.Logger.Error(
			"Unable to consume records",
			zap.Error(err),
		)
		fmr.sendResponse(w, requestID, statusCode, err)
		return
	}
	fmr.sendResponse(w, requestID, http.StatusOK, nil)
}

// validate checks the Firehose access key in the header against
// the one passed into the Config
func (fmr *firehoseReceiver) validate(r *http.Request) (int, error) {
	if string(fmr.config.AccessKey) == "" {
		// No access key is configured - accept all requests.
		return http.StatusAccepted, nil
	}
	if accessKey := r.Header.Get(headerFirehoseAccessKey); accessKey == string(fmr.config.AccessKey) {
		return http.StatusAccepted, nil
	}
	return http.StatusUnauthorized, errInvalidAccessKey
}

// getCommonAttributes unmarshalls the common attributes from the request header
func (*firehoseReceiver) getCommonAttributes(r *http.Request) (map[string]string, error) {
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
	w.Header().Set(headerContentType, "application/json")
	w.Header().Set(headerContentLength, strconv.Itoa(len(payload)))
	w.WriteHeader(statusCode)
	if _, err = w.Write(payload); err != nil {
		fmr.settings.Logger.Error("Failed to send response", zap.Error(err))
	}
}

// loadEncodingExtension tries to load an available extension for the given encoding.
func loadEncodingExtension[T any](host component.Host, encoding, signalType string) (T, error) {
	var zero T
	extensionID, err := encodingToComponentID(encoding)
	if err != nil {
		return zero, err
	}
	encodingExtension, ok := host.GetExtensions()[*extensionID]
	if !ok {
		return zero, fmt.Errorf("unknown encoding extension %q", encoding)
	}
	unmarshaler, ok := encodingExtension.(T)
	if !ok {
		return zero, fmt.Errorf("extension %q is not a %s unmarshaler", encoding, signalType)
	}
	return unmarshaler, nil
}

// encodingToComponentID attempts to parse the encoding string as a component ID.
func encodingToComponentID(encoding string) (*component.ID, error) {
	var id component.ID
	if err := id.UnmarshalText([]byte(encoding)); err != nil {
		return nil, fmt.Errorf("invalid component type: %w", err)
	}
	return &id, nil
}
