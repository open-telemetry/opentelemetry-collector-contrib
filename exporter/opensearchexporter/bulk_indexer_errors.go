// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter"

import (
	"errors"
	"slices"

	"github.com/opensearch-project/opensearch-go/v4"
	"go.opentelemetry.io/collector/consumer/consumererror"
)

// isRetryableResponse reports whether a failed bulk request got an HTTP error
// response worth retrying: a 429 or 5xx from OpenSearch, or an empty or
// non-JSON body. The latter comes from a proxy or load balancer in front of
// OpenSearch (502/503/504 error pages) rather than from OpenSearch itself, and
// opensearch-go does not keep the status code for it. A 2xx body that cannot
// be decoded also lands there; retrying it can duplicate documents but never
// drops them.
func isRetryableResponse(err error) bool {
	if e, ok := errors.AsType[*opensearch.StructError](err); ok {
		return shouldRetryEvent(e.Status)
	}
	if e, ok := errors.AsType[*opensearch.StringError](err); ok {
		return shouldRetryEvent(e.Status)
	}
	return errors.Is(err, opensearch.ErrUnexpectedEmptyBody) ||
		errors.Is(err, opensearch.ErrJSONUnmarshalBody) ||
		errors.Is(err, opensearch.ErrReadBody)
}

// joinBulkErrors builds the error returned for one push.
//
// exporterhelper drops the whole request when any error in the tree is
// permanent, and on retry it keeps only the data of the first consumererror
// it finds. So once something in the push is worth retrying, permanent errors
// are kept as text only, and every item rejected with a retryable status is
// returned in one consumererror built by withRetryData.
//
// requestErr is a retryable error for a whole bulk request. Any other
// non-permanent error in errs also means a whole request failed. In both cases
// the full push is retried, because the failed request's items are not known.
func joinBulkErrors(errs []error, requestErr error, retryErrs []error, withRetryData func(error) error) error {
	if requestErr == nil && len(retryErrs) == 0 {
		return errors.Join(errs...)
	}

	var retryable, permanent []error
	if requestErr != nil {
		retryable = append(retryable, requestErr)
	}
	for _, err := range errs {
		switch {
		case requestErr != nil && errors.Is(err, requestErr):
			// The same failed request, reported again through OnError.
		case consumererror.IsPermanent(err):
			permanent = append(permanent, err)
		default:
			retryable = append(retryable, err)
		}
	}

	err := errors.Join(slices.Concat(retryable, retryErrs)...)
	if len(permanent) > 0 {
		err = errors.Join(err, errors.New(errors.Join(permanent...).Error()))
	}
	if len(retryable) > 0 {
		return err
	}
	return withRetryData(err)
}
