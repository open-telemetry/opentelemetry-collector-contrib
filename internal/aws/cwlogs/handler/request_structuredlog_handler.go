// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package handler // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/cwlogs/handler"

import "github.com/aws/aws-sdk-go/aws/request"

// RequestStructuredLogHandler emf header
var RequestStructuredLogHandler = request.NamedHandler{Name: "RequestStructuredLogHandler", Fn: AddStructuredLogHeader}

// AddStructuredLogHeader emf log
func AddStructuredLogHeader(req *request.Request) {
	req.HTTPRequest.Header.Set("x-amzn-logs-format", "json/emf")
}
