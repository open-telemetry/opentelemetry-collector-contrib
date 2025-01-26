// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsxray

// import (
// 	"net/http"
// 	"testing"

// 	"github.com/aws/aws-sdk-go-v2/aws"
// 	"github.com/stretchr/testify/assert"
// 	"go.opentelemetry.io/collector/component"
// 	"go.uber.org/zap"
// )

// func TestUserAgent(t *testing.T) {
// 	logger := zap.NewNop()

// 	buildInfo := component.BuildInfo{
// 		Command: "test-collector-contrib",
// 		Version: "1.0",
// 	}

// 	xray := NewXRayClient(logger, aws.Config{}, buildInfo).(*xrayClient)
// 	x := xray.xRay

// 	req := request.New(aws.Config{}, metadata.ClientInfo{}, x.Handlers, nil, &request.Operation{
// 		HTTPMethod: http.MethodGet,
// 		HTTPPath:   "/",
// 	}, nil, nil)

// 	x.Handlers.Build.Run(req)
// 	assert.Contains(t, req.HTTPRequest.UserAgent(), "test-collector-contrib/1.0")
// 	assert.Contains(t, req.HTTPRequest.UserAgent(), "xray-otel-exporter/")
// 	assert.Contains(t, req.HTTPRequest.UserAgent(), "exec-env/")
// 	assert.Contains(t, req.HTTPRequest.UserAgent(), "OS/")
// }
