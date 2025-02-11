// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsmiddleware // import "github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsmiddleware"

import (
	"context"
	"encoding"
	"errors"
	"fmt"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/smithy-go/middleware"
	"go.opentelemetry.io/collector/extension"
)

var (
	errNotFound            = errors.New("middleware not found")
	errNotMiddleware       = errors.New("extension is not an AWS middleware")
	errInvalidHandler      = errors.New("invalid handler")
	errUnsupportedPosition = errors.New("unsupported position")
	errUnsupportedVersion  = errors.New("unsupported SDK version")
)

// HandlerPosition is the relative position of a handler used during insertion.
type HandlerPosition int

var (
	_ encoding.TextMarshaler   = (*HandlerPosition)(nil)
	_ encoding.TextUnmarshaler = (*HandlerPosition)(nil)
)

const (
	After HandlerPosition = iota
	Before

	afterStr  = "after"
	beforeStr = "before"
)

// String returns the string representation of the position.
// Returns an empty string if position is unsupported.
func (h HandlerPosition) String() string {
	switch h {
	case Before:
		return beforeStr
	case After:
		return afterStr
	default:
		return ""
	}
}

// MarshalText converts the position into a byte slice.
// Returns an error if unsupported.
func (h HandlerPosition) MarshalText() (text []byte, err error) {
	s := h.String()
	if s == "" {
		return nil, fmt.Errorf("%w: %[2]T(%[2]d)", errUnsupportedPosition, h)
	}
	return []byte(h.String()), nil
}

// UnmarshalText converts the string into a position. Returns an error
// if unsupported.
func (h *HandlerPosition) UnmarshalText(text []byte) error {
	switch s := string(text); s {
	case afterStr:
		*h = After
	case beforeStr:
		*h = Before
	default:
		return fmt.Errorf("%w: %s", errUnsupportedPosition, s)
	}
	return nil
}

// handlerConfig is used to differentiate between handlers and determine
// relative positioning within their groups.
type handlerConfig interface {
	// ID must be unique. It cannot clash with existing middleware.
	ID() string
	// Position to insert the handler.
	Position() HandlerPosition
}

// RequestHandler allows for custom processing of requests.
type RequestHandler interface {
	handlerConfig
	HandleRequest(ctx context.Context, r *http.Request)
}

// ResponseHandler allows for custom processing of responses.
type ResponseHandler interface {
	handlerConfig
	HandleResponse(ctx context.Context, r *http.Response)
}

// Middleware defines the request and response handlers to be configured
// on AWS Clients.
type Middleware interface {
	Handlers() ([]RequestHandler, []ResponseHandler)
}

// Extension is an extension that implements Middleware.
type Extension interface {
	extension.Extension
	Middleware
}

// Configurer provides functions for applying request/response handlers
// to the AWS SDKs.
type Configurer struct {
	requestHandlers  []RequestHandler
	responseHandlers []ResponseHandler
}

// NewConfigurer sets the request/response handlers.
func NewConfigurer(requestHandlers []RequestHandler, responseHandlers []ResponseHandler) *Configurer {
	return &Configurer{requestHandlers: requestHandlers, responseHandlers: responseHandlers}
}

// Configure configures the handlers on the provided AWS SDK.
func (c Configurer) Configure(sdkVersion SDKVersion) error {
	switch v := sdkVersion.(type) {
	case sdkVersion1:
		return c.configureSDKv1(v.handlers)
	case sdkVersion2:
		return c.configureSDKv2(v.cfg)
	default:
		return fmt.Errorf("%w: %T", errUnsupportedVersion, v)
	}
}

// configureSDKv1 adds middleware to the AWS SDK v1. Request handlers are added to the
// Build handler list and response handlers are added to the ValidateResponse handler list.
// Build will only be run once per request, but if there are errors, ValidateResponse will
// be run again for each configured retry.
func (c Configurer) configureSDKv1(handlers *request.Handlers) error {
	var errs error
	for _, handler := range c.requestHandlers {
		if err := appendHandler(&handlers.Build, namedRequestHandler(handler), handler.Position()); err != nil {
			errs = errors.Join(errs, fmt.Errorf("%w (%q): %w", errInvalidHandler, handler.ID(), err))
		}
	}
	for _, handler := range c.responseHandlers {
		if err := appendHandler(&handlers.ValidateResponse, namedResponseHandler(handler), handler.Position()); err != nil {
			errs = errors.Join(errs, fmt.Errorf("%w (%q): %w", errInvalidHandler, handler.ID(), err))
		}
	}
	return errs
}

// configureSDKv2 adds middleware to the AWS SDK v2. Request handlers are added to the
// Build step and response handlers are added to the Deserialize step.
func (c Configurer) configureSDKv2(config *aws.Config) error {
	var errs error
	for _, handler := range c.requestHandlers {
		relativePosition, err := toRelativePosition(handler.Position())
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("%w (%q): %w", errInvalidHandler, handler.ID(), err))
			continue
		}
		config.APIOptions = append(config.APIOptions, withBuildOption(&requestMiddleware{RequestHandler: handler}, relativePosition))
	}
	for _, handler := range c.responseHandlers {
		relativePosition, err := toRelativePosition(handler.Position())
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("%w (%q): %w", errInvalidHandler, handler.ID(), err))
			continue
		}
		config.APIOptions = append(config.APIOptions, withDeserializeOption(&responseMiddleware{ResponseHandler: handler}, relativePosition))
	}
	return errs
}

// appendHandler adds the handler to the list based on the position.
func appendHandler(handlerList *request.HandlerList, handler request.NamedHandler, position HandlerPosition) error {
	relativePosition, err := toRelativePosition(position)
	if err != nil {
		return err
	}
	switch relativePosition {
	case middleware.Before:
		handlerList.PushFrontNamed(handler)
	case middleware.After:
		handlerList.PushBackNamed(handler)
	}
	return nil
}

// toRelativePosition maps the HandlerPosition to a middleware.RelativePosition. It also validates that
// the HandlerPosition provided is supported and returns an errUnsupportedPosition if it isn't.
func toRelativePosition(position HandlerPosition) (middleware.RelativePosition, error) {
	switch position {
	case Before:
		return middleware.Before, nil
	case After:
		return middleware.After, nil
	default:
		return -1, fmt.Errorf("%w: %s", errUnsupportedPosition, position)
	}
}
