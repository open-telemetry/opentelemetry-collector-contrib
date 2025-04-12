// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opampextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/opampextension"

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension/extensionauth"
	"go.uber.org/zap"
)

// headerCaptureRoundTripper is a RoundTripper that captures the headers of the request
// that passes through it.
type headerCaptureRoundTripper struct {
	lastHeader http.Header
}

func (h *headerCaptureRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	h.lastHeader = req.Header.Clone()
	// Dummy response is recorded here
	return &http.Response{
		Status:     "200 OK",
		StatusCode: http.StatusOK,
		Proto:      "HTTP/1.0",
		ProtoMajor: 1,
		ProtoMinor: 0,
		Body:       io.NopCloser(&bytes.Buffer{}),
		Request:    req,
	}, nil
}

func makeHeadersFunc(logger *zap.Logger, serverCfg *OpAMPServer, host component.Host) (func(http.Header) http.Header, error) {
	var emptyComponentID component.ID
	if serverCfg == nil || serverCfg.GetAuthExtensionID() == emptyComponentID {
		return nil, nil
	}

	extID := serverCfg.GetAuthExtensionID()
	ext, ok := host.GetExtensions()[extID]
	if !ok {
		return nil, fmt.Errorf("could not find auth extension %q", extID)
	}

	authExt, ok := ext.(extensionauth.HTTPClient)
	if !ok {
		return nil, fmt.Errorf("auth extension %q is not an extensionauth.HTTPClient", extID)
	}

	hcrt := &headerCaptureRoundTripper{}
	rt, err := authExt.RoundTripper(hcrt)
	if err != nil {
		return nil, fmt.Errorf("could not create roundtripper for authentication: %w", err)
	}

	return func(h http.Header) http.Header {
		// This is a workaround while websocket authentication is being worked on.
		// Currently, we are waiting on the auth module to be stabilized.
		// See for more info: https://github.com/open-telemetry/opentelemetry-collector/issues/10864
		dummyReq, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
		if err != nil {
			logger.Error("Failed to create dummy request for authentication.", zap.Error(err))
			return h
		}

		dummyReq.Header = h

		_, err = rt.RoundTrip(dummyReq)
		if err != nil {
			logger.Error("Error while performing round-trip for authentication.", zap.Error(err))
			return h
		}

		return hcrt.lastHeader
	}, nil
}
