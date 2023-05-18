// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"net/http"
	"testing"

	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/stretchr/testify/assert"
)

func TestAddUserAgentCWAgent(t *testing.T) {
	httpReq, _ := http.NewRequest(http.MethodPost, "", nil)
	r := &request.Request{
		HTTPRequest: httpReq,
		Body:        nil,
	}
	r.SetBufferBody([]byte{})

	AddStructuredLogHeader(r)

	structuredLogHeader := r.HTTPRequest.Header.Get("x-amzn-logs-format")
	assert.Equal(t, "json/emf", structuredLogHeader)
}
