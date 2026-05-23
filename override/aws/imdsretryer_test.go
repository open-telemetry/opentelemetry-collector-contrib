// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package aws

import (
	"errors"
	"net/http"
	"testing"

	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/stretchr/testify/assert"
)

func TestIMDSRetryer_IsErrorRetryable(t *testing.T) {
	tests := map[string]struct {
		err  error
		want bool
	}{
		"NilNotRetryable": {
			err:  nil,
			want: false,
		},
		"IMDSResponseError404Retryable": {
			err: &smithyhttp.ResponseError{
				Response: &smithyhttp.Response{
					Response: &http.Response{StatusCode: http.StatusNotFound},
				},
				Err: errors.New("request to EC2 IMDS failed"),
			},
			want: true,
		},
		"IMDSResponseError500Retryable": {
			err: &smithyhttp.ResponseError{
				Response: &smithyhttp.Response{
					Response: &http.Response{StatusCode: http.StatusInternalServerError},
				},
				Err: errors.New("request to EC2 IMDS failed"),
			},
			want: true,
		},
		"WrappedIMDSResponseError503Retryable": {
			// errors.As must unwrap joined errors.
			err: errors.Join(
				errors.New("outer error"),
				&smithyhttp.ResponseError{
					Response: &smithyhttp.Response{
						Response: &http.Response{StatusCode: http.StatusServiceUnavailable},
					},
					Err: errors.New("request to EC2 IMDS failed"),
				},
			),
			want: true,
		},
		"GenericErrorNotRetryable": {
			err:  errors.New("some other error"),
			want: false,
		},
	}

	retryer := NewIMDSRetryer(DefaultIMDSRetries)

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := retryer.IsErrorRetryable(tt.err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIMDSRetryer_MaxAttempts(t *testing.T) {
	// NewIMDSRetryer(n).MaxAttempts() == n + 1 — v2 counts the first attempt.
	tests := map[string]struct {
		retries int
		want    int
	}{
		"DefaultRetries": {retries: DefaultIMDSRetries, want: DefaultIMDSRetries + 1},
		"ZeroRetries":    {retries: 0, want: 1},
		"TwoRetries":     {retries: 2, want: 3},
		"FiveRetries":    {retries: 5, want: 6},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			r := NewIMDSRetryer(tt.retries)
			assert.Equal(t, tt.want, r.MaxAttempts())
		})
	}
}

func TestGetDefaultRetryNumber(t *testing.T) {
	tests := map[string]struct {
		envValue string
		want     int
	}{
		"Unset":         {envValue: "", want: DefaultIMDSRetries},
		"ValidZero":     {envValue: "0", want: 0},
		"ValidPositive": {envValue: "5", want: 5},
		"Negative":      {envValue: "-1", want: DefaultIMDSRetries},
		"NonNumeric":    {envValue: "abc", want: DefaultIMDSRetries},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Setenv(EnvIMDSNumberRetry, tt.envValue)
			assert.Equal(t, tt.want, GetDefaultRetryNumber())
		})
	}
}
