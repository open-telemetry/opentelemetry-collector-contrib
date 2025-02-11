// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsxray

import (
	"net/http"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client/metadata"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
)

func TestUserAgent(t *testing.T) {
	logger := zap.NewNop()

	buildInfo := component.BuildInfo{
		Command: "test-collector-contrib",
		Version: "1.0",
	}

	newSession, err := session.NewSession()
	require.NoError(t, err)
	xray := NewXRayClient(logger, &aws.Config{}, buildInfo, newSession).(*xrayClient)

	req := request.New(aws.Config{}, metadata.ClientInfo{}, *xray.Handlers(), nil, &request.Operation{
		HTTPMethod: http.MethodGet,
		HTTPPath:   "/",
	}, nil, nil)

	xray.Handlers().Build.Run(req)
	assert.Contains(t, req.HTTPRequest.UserAgent(), "test-collector-contrib/1.0")
	assert.Contains(t, req.HTTPRequest.UserAgent(), "xray-otel-exporter/")
	assert.Contains(t, req.HTTPRequest.UserAgent(), "exec-env/")
	assert.Contains(t, req.HTTPRequest.UserAgent(), "OS/")
}
