// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package extensions // import "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/extensions"

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension/extensionauth"
	"go.uber.org/zap"
)

// headerCaptureRoundTripper captures the headers set on a request by an auth
// extension's RoundTripper. Adapted from the opamp extension, which keeps
// this type unexported.
type headerCaptureRoundTripper struct {
	lastHeader http.Header
}

func (h *headerCaptureRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	h.lastHeader = req.Header.Clone()
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

// MakeHeadersFunc returns a function that augments OpAMP request headers with
// headers produced by the auth extension referenced by authID. The returned
// function preserves the incoming static headers and overlays whatever the
// auth extension's RoundTripper sets.
//
// It errors at startup when the extension cannot be found or does not
// implement extensionauth.HTTPClient. Errors at call time (for example a
// failing token refresh) are logged and the original headers are returned so
// the supervisor can keep running.
func MakeHeadersFunc(
	logger *zap.Logger,
	authID component.ID,
	exts map[component.ID]component.Component,
) (func(http.Header) http.Header, error) {
	ext, ok := exts[authID]
	if !ok {
		return nil, fmt.Errorf("auth extension %q not found", authID)
	}

	authExt, ok := ext.(extensionauth.HTTPClient)
	if !ok {
		return nil, fmt.Errorf("extension %q does not implement extensionauth.HTTPClient", authID)
	}

	hcrt := &headerCaptureRoundTripper{}
	rt, err := authExt.RoundTripper(hcrt)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth round tripper for %q: %w", authID, err)
	}

	return func(h http.Header) http.Header {
		// Websocket auth doesn't support RoundTripper directly, so the opamp
		// extension uses a dummy request as a workaround. See
		// https://github.com/open-telemetry/opentelemetry-collector/issues/10864
		dummyReq, err := http.NewRequest(http.MethodGet, "http://example.com", http.NoBody)
		if err != nil {
			logger.Error("Failed to create dummy request for authentication.", zap.Error(err))
			return h
		}
		dummyReq.Header = h

		if _, err := rt.RoundTrip(dummyReq); err != nil {
			logger.Error("Error while performing round-trip for authentication.", zap.Error(err))
			return h
		}
		return hcrt.lastHeader
	}, nil
}
