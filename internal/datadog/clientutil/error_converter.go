// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clientutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog/clientutil"

import (
	"net/http"

	"go.opentelemetry.io/collector/consumer/consumererror"
)

// WrapError wraps an error to a permanent consumer error that won't be retried if the http response code is non-retriable.
func WrapError(err error, resp *http.Response) error {
	if err == nil || resp == nil || !isNonRetriable(resp) {
		return err
	}
	return consumererror.NewPermanent(err)
}

func isNonRetriable(resp *http.Response) bool {
	return resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusRequestEntityTooLarge || resp.StatusCode == http.StatusForbidden
}
