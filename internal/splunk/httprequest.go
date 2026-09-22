// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunk // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const HeaderRetryAfter = "Retry-After"

// HandleHTTPCode handles an http response and returns the right type of error in case of a failure.
func HandleHTTPCode(resp *http.Response) error {
	// Splunk accepts all 2XX codes.
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		return nil
	}

	err := fmt.Errorf(
		"HTTP %q %d %q",
		resp.Request.URL.Path,
		resp.StatusCode,
		http.StatusText(resp.StatusCode),
	)

	switch {
	// Check for responses that may include "Retry-After" header.
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable:
		// Fallback to 0 if the Retry-After header is not present. This will trigger the
		// default backoff policy by our caller (retry handler).
		retryAfter := 0
		if val := resp.Header.Get(HeaderRetryAfter); val != "" {
			if seconds, err2 := strconv.Atoi(val); err2 == nil {
				retryAfter = seconds
			}
		}
		// Indicate to our caller to pause for the specified number of seconds.
		err = exporterhelper.NewThrottleRetry(err, time.Duration(retryAfter)*time.Second)
	// 408 Request Timeout is transient; leave it as a plain (retryable) error.
	case resp.StatusCode == http.StatusRequestTimeout:
	// All other client errors (4xx) are permanent: retrying an identical request
	// cannot fix a malformed, unauthorized, oversized, or unsupported request.
	case resp.StatusCode >= http.StatusBadRequest && resp.StatusCode < http.StatusInternalServerError:
		err = consumererror.NewPermanent(err)
	}

	// Server errors (5xx) and anything else fall through as a plain (retryable) error.
	return err
}
