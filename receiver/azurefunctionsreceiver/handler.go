// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azurefunctionsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azurefunctionsreceiver"

import (
	"fmt"
	"io"
	"net/http"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azurefunctionsreceiver/internal/handler"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azurefunctionsreceiver/internal/protocol"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azurefunctionsreceiver/internal/transport"
)

// Protocol parses the Azure Functions invoke HTTP body into a ParsedRequest and
// writes success or failure responses. Different triggers may share the same
// protocol (e.g. invoke envelope + transport decode) or use a different one.
type Protocol interface {
	ParseRequest(methodName string, body []byte) (handler.ParsedRequest, error)
	Success(w http.ResponseWriter)
	Failure(w http.ResponseWriter, err error, body []byte)
}

// Profile binds a method name (binding/path) to a Protocol and Consumer.
// The generic HTTP handler uses it to: parse request with protocol, then consume with consumer.
type Profile struct {
	method   string
	protocol Protocol
	consumer handler.Consumer
}

// NewProfile returns a Profile for the given method, protocol, and consumer.
func NewProfile(method string, protocol Protocol, consumer handler.Consumer) Profile {
	return Profile{
		method:   method,
		protocol: protocol,
		consumer: consumer,
	}
}

// MetadataExtractor turns raw invoke Metadata JSON into resource attributes.
// Trigger-specific (e.g. Event Hub, HTTP); pass nil for no metadata extraction.
type MetadataExtractor func(raw []byte) map[string]string

// invokeProtocol implements Protocol for the Azure Functions invoke envelope:
// parse body, decode Data[methodName] via transport, optionally fill Metadata using an extractor.
type invokeProtocol struct {
	decoder   *transport.BinaryDecoder
	logger    *zap.Logger
	extractor MetadataExtractor
}

// NewInvokeProtocol returns a Protocol that parses invoke requests and decodes the binding
// payload. If extractor is non-nil and invoke Metadata is present, it is used to fill ParsedRequest.Metadata.
func NewInvokeProtocol(decoder *transport.BinaryDecoder, logger *zap.Logger, extractor MetadataExtractor) Protocol {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &invokeProtocol{
		decoder:   decoder,
		logger:    logger,
		extractor: extractor,
	}
}

// ParseRequest implements Protocol.
func (p *invokeProtocol) ParseRequest(methodName string, body []byte) (handler.ParsedRequest, error) {
	invokeReq, err := protocol.ParseInvokeRequest(body)
	if err != nil {
		return handler.ParsedRequest{}, err
	}
	data, ok := invokeReq.Data[methodName]
	if !ok {
		return handler.ParsedRequest{}, fmt.Errorf("missing data for binding %q", methodName)
	}
	content, err := p.decoder.Decode(data)
	if err != nil {
		return handler.ParsedRequest{}, fmt.Errorf("decode payload: %w", err)
	}
	var metadata map[string]string
	if p.extractor != nil && len(invokeReq.Metadata) > 0 {
		metadata = p.extractor(invokeReq.Metadata)
	}
	return handler.ParsedRequest{Content: content, Metadata: metadata}, nil
}

// Success implements Protocol.
func (p *invokeProtocol) Success(w http.ResponseWriter) {
	protocol.WriteSuccess(w)
}

// Failure implements Protocol.
func (p *invokeProtocol) Failure(w http.ResponseWriter, err error, body []byte) {
	protocol.WriteFailure(w, err, body)
	p.logger.Error("request failed", zap.Error(err))
}

// createHandler returns an HTTP handler for the given profile. It reads the body,
// parses it with the protocol, consumes with the consumer, and responds with success or failure.
// The same flow is used for every trigger; no trigger-specific logic here.
func createHandler(profile Profile) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			profile.protocol.Failure(w, fmt.Errorf("read body: %w", err), nil)
			return
		}
		defer r.Body.Close()

		parsed, err := profile.protocol.ParseRequest(profile.method, body)
		if err != nil {
			profile.protocol.Failure(w, err, body)
			return
		}

		if err := profile.consumer.ConsumeEvents(r.Context(), parsed); err != nil {
			profile.protocol.Failure(w, err, body)
			return
		}

		profile.protocol.Success(w)
	})
}
