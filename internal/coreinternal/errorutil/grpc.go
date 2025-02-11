// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package errorutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/errorutil"

import (
	"go.opentelemetry.io/collector/consumer/consumererror"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func GrpcError(err error) error {
	s, ok := status.FromError(err)
	if !ok {
		// Default to a retryable error
		// https://github.com/open-telemetry/opentelemetry-proto/blob/main/docs/specification.md#failures
		code := codes.Unavailable
		if consumererror.IsPermanent(err) {
			// non-retryable error
			code = codes.Unknown
		}
		s = status.New(code, err.Error())
	}
	return s.Err()
}
