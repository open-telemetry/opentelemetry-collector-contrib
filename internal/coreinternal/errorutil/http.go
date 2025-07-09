// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package errorutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/errorutil"

import (
	"net/http"

	"go.opentelemetry.io/collector/consumer/consumererror"
)

func HTTPError(w http.ResponseWriter, err error) {
	if err == nil {
		return
	}
	http.Error(w, err.Error(), GetHTTPStatusCodeFromError(err))
}

func GetHTTPStatusCodeFromError(err error) int {
	// See requirements for receivers
	// https://github.com/open-telemetry/opentelemetry-collector/blob/8e522ad950de6326a0841d7e1bef808bbc0d3537/receiver/doc.go#L10-L29

	// See https://github.com/open-telemetry/opentelemetry-proto/blob/main/docs/specification.md#failures-1
	// to see a list of retryable http status codes.

	// non-retryable status
	status := http.StatusBadRequest
	if !consumererror.IsPermanent(err) {
		// retryable status
		status = http.StatusServiceUnavailable
	}
	return status
}
