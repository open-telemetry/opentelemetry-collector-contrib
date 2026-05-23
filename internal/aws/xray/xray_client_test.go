// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsxray

import (
	"context"
	"runtime"
	"testing"

	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/stretchr/testify/assert"
)

func TestAddToUserAgentHeader(t *testing.T) {
	tests := []struct {
		name         string
		userAgentVal string
		pos          middleware.RelativePosition
	}{
		{
			name:         "tracing.XRayVersionUserAgentHandler",
			userAgentVal: agentPrefix + getModVersion() + execEnvPrefix + "UNKNOWN" + osPrefix + runtime.GOOS + "-" + runtime.GOARCH,
			pos:          middleware.After,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stack := middleware.NewStack(test.name, func() any { return struct{}{} })

			err := stack.Serialize.Add(&addToUserAgentHeader{
				id:  test.name,
				val: test.userAgentVal,
			}, test.pos)
			assert.NoError(t, err)

			err = stack.Serialize.Add(middleware.SerializeMiddlewareFunc("TestCapture", func(
				_ context.Context,
				in middleware.SerializeInput,
				_ middleware.SerializeHandler,
			) (out middleware.SerializeOutput, metadata middleware.Metadata, err error) {
				req := in.Request.(*smithyhttp.Request)
				assert.Contains(t, req.Header.Get("User-Agent"), test.userAgentVal)
				assert.Equal(t, "123456.789", req.Header.Get("X-Amzn-Xray-Timestamp"))
				return out, metadata, nil
			}), middleware.After)
			assert.NoError(t, err)
		})
	}
}
