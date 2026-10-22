// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafka

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestAWSMSKCredentialRefreshFailureLogging(t *testing.T) {
	const sensitive = "sensitive-detail-do-not-log"
	for _, tt := range []struct {
		name string
		err  error
		kind string
		code string
	}{
		{
			name: "wrapped deadline", kind: "timeout",
			err: fmt.Errorf("%s: %w", sensitive, context.DeadlineExceeded),
		},
		{
			name: "network timeout", kind: "timeout",
			err: &url.Error{
				Op: "Post", URL: "https://example.invalid/" + sensitive,
				Err: &net.DNSError{Err: sensitive, IsTimeout: true},
			},
		},
		{
			name: "network failure", kind: "network",
			err: fmt.Errorf("%s: %w", sensitive, &net.OpError{Op: "dial", Net: "tcp", Err: errors.New(sensitive)}),
		},
		{
			name: "SDK request failure", kind: "network",
			err: &smithy.OperationError{
				ServiceID: "STS", OperationName: "AssumeRoleWithWebIdentity",
				Err: &smithyhttp.RequestSendError{Err: errors.New(sensitive)},
			},
		},
		{
			name: "access denied", kind: "sts_api_error", code: "AccessDenied",
			err: &smithy.GenericAPIError{Code: "AccessDenied", Message: sensitive},
		},
		{
			name: "wrapped STS error", kind: "sts_api_error", code: "Throttling",
			err: &smithy.OperationError{
				ServiceID: "STS", OperationName: "AssumeRoleWithWebIdentity",
				Err: &smithy.GenericAPIError{Code: "Throttling", Message: sensitive},
			},
		},
		{
			name: "invalid identity token", kind: "sts_api_error", code: "InvalidIdentityToken",
			err: &smithy.GenericAPIError{Code: "InvalidIdentityToken", Message: sensitive},
		},
		{
			name: "expired token", kind: "sts_api_error", code: "ExpiredTokenException",
			err: &smithy.GenericAPIError{Code: "ExpiredTokenException", Message: sensitive},
		},
		{
			name: "disabled region", kind: "sts_api_error", code: "RegionDisabledException",
			err: &smithy.GenericAPIError{Code: "RegionDisabledException", Message: sensitive},
		},
		{
			name: "unrecognized STS code", kind: "sts_api_error", code: "unknown",
			err: &smithy.GenericAPIError{Code: sensitive, Message: sensitive},
		},
		{
			name: "token file error", kind: "unknown",
			err: &os.PathError{Op: "read", Path: sensitive, Err: os.ErrNotExist},
		},
		{name: "unknown error", kind: "unknown", err: errors.New(sensitive)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			logs, observed := observer.New(zap.DebugLevel)
			credentials := aws.Credentials{
				AccessKeyID: "access-key-do-not-log", SecretAccessKey: "secret-key-do-not-log",
				SessionToken: "session-token-do-not-log", CanExpire: true, Expires: time.Now().Add(time.Minute),
			}
			var calls int
			provider := newAWSMSKRefreshProvider(aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
				calls++
				if calls == 1 {
					return credentials, nil
				}
				return aws.Credentials{}, tt.err
			}), zap.New(logs))
			cache := aws.NewCredentialsCache(provider, configureAWSMSKCache)
			_, err := cache.Retrieve(t.Context())
			require.NoError(t, err)
			// These credentials are inside the early-refresh window but still
			// valid at AWS. Fallback succeeds and absorbs the refresh error.
			fallback, err := cache.Retrieve(t.Context())
			require.NoError(t, err)
			assert.Equal(t, credentials.AccessKeyID, fallback.AccessKeyID)
			entries := observed.FilterMessage("AWS MSK credential refresh failed").All()
			require.Len(t, entries, 1)
			assert.Equal(t, zap.WarnLevel, entries[0].Level)
			fields := entries[0].ContextMap()
			assert.Equal(t, tt.kind, fields["error_kind"])
			if tt.code == "" {
				assert.NotContains(t, fields, "aws_error_code")
			} else {
				assert.Equal(t, tt.code, fields["aws_error_code"])
			}
			for _, value := range []string{sensitive, credentials.AccessKeyID, credentials.SecretAccessKey, credentials.SessionToken} {
				assert.NotContains(t, fmt.Sprint(observed.All()), value)
			}
		})
	}
}
