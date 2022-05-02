package lmexporter

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"

	"net/http"
	"runtime"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/model/otlp"
	"go.opentelemetry.io/collector/model/pdata"

	"google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/protobuf/proto"

	"go.uber.org/zap"
)

type tracesExporter struct {
	config *Config
	client HttpClient
	logger *zap.Logger

	// Default user-agent header.
	userAgent string
}

const (
	headerRetryAfter         = "Retry-After"
	maxHTTPResponseReadBytes = 64 * 1024
	traceIngestURI           = "/api/v1/traces"
	traceBaseURI             = "/rest"
)

// Crete new exporter.
func newTracesExporter(cfg *Config, logger *zap.Logger, buildInfo component.BuildInfo) (*tracesExporter, error) {

	newClient := NewLMHTTPClient(cfg.APIToken, cfg.Headers, true)

	userAgent := fmt.Sprintf("%s/%s (%s/%s)",
		buildInfo.Description, buildInfo.Version, runtime.GOOS, runtime.GOARCH)

	// client construction is deferred to start
	return &tracesExporter{
		config:    cfg,
		logger:    logger,
		client:    newClient,
		userAgent: userAgent,
	}, nil
}

func (e *tracesExporter) pushTraces(ctx context.Context, td pdata.Traces) error {
	tracesMarshaler := otlp.NewProtobufTracesMarshaler()
	request, err := tracesMarshaler.MarshalTraces(td)
	if err != nil {
		return err
	}
	return e.export(ctx, e.config.URL, request)
}

func (e *tracesExporter) export(ctx context.Context, url string, request []byte) error {
	e.logger.Debug("Preparing to make HTTP Traces request", zap.String("url", url))
	headers := make(map[string]string)
	headers["Content-Type"] = "application/x-protobuf"

	resp, err := e.client.MakeRequest("3", http.MethodPost, traceBaseURI, traceIngestURI, "", 5*time.Second, bytes.NewBuffer(request), headers)
	if err != nil {
		return err
	}
	defer func() {
		// Discard any remaining response body when we are done reading.
		io.CopyN(ioutil.Discard, bytes.NewReader(resp.Body), maxHTTPResponseReadBytes) // nolint:errcheck
	}()

	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		// Request is successful.
		return nil
	}

	respStatus := readResponse(resp)

	// Format the error message. Use the status if it is present in the response.
	var formattedErr error
	if respStatus != nil {
		formattedErr = fmt.Errorf(
			"error exporting items, request to %s responded with HTTP Status Code %d, Message=%s, Details=%v",
			url, resp.StatusCode, respStatus.Message, respStatus.Details)
	} else {
		formattedErr = fmt.Errorf(
			"error exporting items, request to %s responded with HTTP Status Code %d",
			url, resp.StatusCode)
	}

	// Check if the server is overwhelmed.
	// See spec https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/protocol/otlp.md#throttling-1
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
		// Fallback to 0 if the Retry-After header is not present. This will trigger the
		// default backoff policy by our caller (retry handler).
		retryAfter := 0
		if val := resp.Headers.Get(headerRetryAfter); val != "" {
			if seconds, err2 := strconv.Atoi(val); err2 == nil {
				retryAfter = seconds
			}
		}
		// Indicate to our caller to pause for the specified number of seconds.
		return exporterhelper.NewThrottleRetry(formattedErr, time.Duration(retryAfter)*time.Second)
	}

	if resp.StatusCode == http.StatusBadRequest {
		// Report the failure as permanent if the server thinks the request is malformed.
		return consumererror.NewPermanent(formattedErr)
	}

	// All other errors are retryable, so don't wrap them in consumererror.Permanent().
	return formattedErr
}

// Read the response and decode the status.Status from the body.
// Returns nil if the response is empty or cannot be decoded.
func readResponse(resp *APIResponse) *status.Status {
	var respStatus *status.Status
	if resp.StatusCode >= 400 && resp.StatusCode <= 599 {
		// Request failed. Read the body. OTLP spec says:
		// "Response body for all HTTP 4xx and HTTP 5xx responses MUST be a
		// Protobuf-encoded Status message that describes the problem."
		maxRead := resp.ContentLength
		if maxRead == -1 || maxRead > maxHTTPResponseReadBytes {
			maxRead = maxHTTPResponseReadBytes
		}
		respBytes := make([]byte, maxRead)
		n, err := io.ReadFull(bytes.NewReader(resp.Body), respBytes)
		if err == nil && n > 0 {
			// Decode it as Status struct. See https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/protocol/otlp.md#failures
			respStatus = &status.Status{}
			err = proto.Unmarshal(respBytes, respStatus)
			if err != nil {
				respStatus = nil
			}
		}
	}
	return respStatus
}
