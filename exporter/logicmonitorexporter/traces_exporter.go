// Copyright The OpenTelemetry Authors
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

package logicmonitorexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logicmonitorexporter"

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"go.uber.org/zap"
	"google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/protobuf/proto"
)

type tracesExporter struct {
	// Input configuration.
	config   *Config
	client   HTTPClient
	logger   *zap.Logger
	settings component.TelemetrySettings
	// Default user-agent header.
	userAgent string
}

const (
	headerRetryAfter         = "Retry-After"
	maxHTTPResponseReadBytes = 64 * 1024
	traceIngestURI           = "/api/v1/traces"
	traceBaseURI             = "/rest"
)

// Create new traces exporter
func newTracesExporter(cfg *Config, set component.ExporterCreateSettings) (*tracesExporter, error) {

	if cfg.URL != "" {
		u, err := url.Parse(cfg.URL)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return nil, fmt.Errorf("endpoint must be a valid URL")
		}
	}

	newClient := NewLMHTTPClient(cfg.APIToken, cfg.Headers)

	userAgent := fmt.Sprintf("%s/%s (%s/%s)",
		set.BuildInfo.Description, set.BuildInfo.Version, runtime.GOOS, runtime.GOARCH)

	return &tracesExporter{
		config:    cfg,
		logger:    set.Logger,
		client:    newClient,
		userAgent: userAgent,
		settings:  set.TelemetrySettings,
	}, nil
}

func (e *tracesExporter) pushTraces(ctx context.Context, td ptrace.Traces) error {
	tr := ptraceotlp.NewExportRequestFromTraces(td)
	request, err := tr.MarshalProto()
	if err != nil {
		return consumererror.NewPermanent(err)
	}

	return e.export(ctx, e.config.URL, request)
}

func (e *tracesExporter) export(ctx context.Context, url string, request []byte) error {
	headers := make(map[string]string)
	headers["Content-Type"] = "application/x-protobuf"

	resp, err := e.client.MakeRequest(ctx, Version3, http.MethodPost, traceBaseURI, traceIngestURI, url, 5*time.Second, bytes.NewBuffer(request), headers)
	if err != nil {
		return err
	}
	defer func() {
		// Discard any remaining response body when we are done reading.
		_, _ = io.CopyN(io.Discard, bytes.NewReader(resp.Body), maxHTTPResponseReadBytes) // nolint:errcheck
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
			url+traceIngestURI, resp.StatusCode, respStatus.Message, respStatus.Details)
	} else {
		formattedErr = fmt.Errorf(
			"error exporting items, request to %s responded with HTTP Status Code %d",
			url+traceIngestURI, resp.StatusCode)
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
