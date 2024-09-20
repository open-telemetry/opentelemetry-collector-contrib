// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package errorutil

import (
	"net/http"

	"go.opentelemetry.io/collector/consumer/consumererror"
)

func HttpError(w http.ResponseWriter, err error) {
	if err == nil {
		return
	}
	// non-retryable status
	status := http.StatusBadRequest
	if !consumererror.IsPermanent(err) {
		// retryable status
		status = http.StatusServiceUnavailable
	}
	http.Error(w, err.Error(), status)
}
