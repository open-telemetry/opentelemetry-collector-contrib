// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package aws // import "github.com/amazon-contributing/opentelemetry-collector-contrib/override/aws"

import (
	"errors"
	"os"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"go.uber.org/zap"
)

const (
	DefaultIMDSRetries = 1
	EnvIMDSNumberRetry = "IMDS_NUMBER_RETRY"
)

// IMDSRetryer extends the SDK v2 standard retryer to treat IMDS errors
// as retryable. The default SDK retryer treats bare IMDS failures
// (which arrive as *smithyhttp.ResponseError without a more specific
// type) as non-retryable; this override adds them to the retryable set
// so a single transient IMDS blip does not abort agent startup.
type IMDSRetryer struct {
	*retry.Standard
	logger *zap.Logger
}

var _ aws.RetryerV2 = (*IMDSRetryer)(nil)

// NewIMDSRetryer returns a retryer that retries up to `retries` times
// in addition to the first attempt.
func NewIMDSRetryer(retries int) *IMDSRetryer {
	r := &IMDSRetryer{
		Standard: retry.NewStandard(func(options *retry.StandardOptions) {
			options.MaxAttempts = retries + 1 // MaxAttempts includes the first attempt
		}),
	}
	if logger, err := zap.NewDevelopment(); err == nil {
		r.logger = logger
	}
	return r
}

// IsErrorRetryable returns true for any error the standard retryer
// considers retryable, plus any error that is or wraps a
// *smithyhttp.ResponseError (the type IMDS middleware uses to surface
// failures).
func (r *IMDSRetryer) IsErrorRetryable(err error) bool {
	var responseErr *smithyhttp.ResponseError
	shouldRetry := errors.As(err, &responseErr) || r.Standard.IsErrorRetryable(err)
	if r.logger != nil {
		r.logger.Debug("imds error : ", zap.Bool("shouldRetry", shouldRetry), zap.Error(err))
	}
	return shouldRetry
}

// GetDefaultRetryNumber reads the IMDS retry count from the
// IMDS_NUMBER_RETRY environment variable, falling back to
// DefaultIMDSRetries when unset, non-numeric, or negative.
func GetDefaultRetryNumber() int {
	if v := os.Getenv(EnvIMDSNumberRetry); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			return n
		}
	}
	return DefaultIMDSRetries
}
