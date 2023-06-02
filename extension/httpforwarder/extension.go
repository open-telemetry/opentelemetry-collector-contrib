// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package httpforwarder // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/httpforwarder"

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"
)

type httpForwarder struct {
	forwardTo  *url.URL
	httpClient *http.Client
	server     *http.Server
	settings   component.TelemetrySettings
	config     *Config
}

var _ extension.Extension = (*httpForwarder)(nil)

func (h *httpForwarder) Start(_ context.Context, host component.Host) error {
	listener, err := h.config.Ingress.ToListener()
	if err != nil {
		return fmt.Errorf("failed to bind to address %s: %w", h.config.Ingress.Endpoint, err)
	}

	httpClient, err := h.config.Egress.ToClient(host, h.settings)
	if err != nil {
		return fmt.Errorf("failed to create HTTP Client: %w", err)
	}
	h.httpClient = httpClient

	handler := http.NewServeMux()
	handler.HandleFunc("/", h.forwardRequest)

	h.server, err = h.config.Ingress.ToServer(host, h.settings, handler)
	if err != nil {
		return fmt.Errorf("failed to create HTTP Client: %w", err)
	}

	go func() {
		if errHTTP := h.server.Serve(listener); !errors.Is(errHTTP, http.ErrServerClosed) && errHTTP != nil {
			host.ReportFatalError(errHTTP)
		}
	}()

	return nil
}

func (h *httpForwarder) Shutdown(_ context.Context) error {
	if h.server == nil {
		return nil
	}
	return h.server.Close()
}

func (h *httpForwarder) forwardRequest(writer http.ResponseWriter, request *http.Request) {
	forwarderRequest := request.Clone(request.Context())
	forwarderRequest.URL.Host = h.forwardTo.Host
	forwarderRequest.URL.Scheme = h.forwardTo.Scheme
	forwarderRequest.Host = h.forwardTo.Host
	// Clear RequestURI to avoid getting "http: Request.RequestURI can't be set in client requests" error.
	forwarderRequest.RequestURI = ""

	// Add additional headers.
	for k, v := range h.config.Egress.Headers {
		forwarderRequest.Header.Add(k, string(v))
	}

	// Add "Via" header for tracking purposes on both the outgoing requests and responses.
	// See https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Via.
	addViaHeader(forwarderRequest.Header, request.Proto, request.Host)

	response, err := h.httpClient.Do(forwarderRequest)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadGateway)
	}

	if response == nil {
		return
	}
	defer response.Body.Close()

	// Copy over response from the final destination.
	for k := range response.Header {
		writer.Header().Set(k, response.Header.Get(k))
	}
	addViaHeader(writer.Header(), response.Proto, request.Host)

	writer.WriteHeader(response.StatusCode)
	written, err := io.Copy(writer, response.Body)
	if err != nil {
		h.settings.Logger.Warn("Error writing HTTP response message", zap.Error(err))
	}

	if response.ContentLength != written {
		h.settings.Logger.Warn("Response from target not fully copied, body might be corrupted")
	}
}

func addViaHeader(header http.Header, protocol string, host string) {
	header.Add("Via", fmt.Sprintf("%s %s", protocol, host))
}

func newHTTPForwarder(config *Config, settings component.TelemetrySettings) (extension.Extension, error) {
	if config.Egress.Endpoint == "" {
		return nil, errors.New("'egress.endpoint' config option cannot be empty")
	}

	var url, err = url.Parse(config.Egress.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("enter a valid URL for 'egress.endpoint': %w", err)
	}

	h := &httpForwarder{
		config:    config,
		forwardTo: url,
		settings:  settings,
	}

	return h, nil
}
