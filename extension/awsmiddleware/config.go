// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsmiddleware // import "github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsmiddleware"

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
)

// getMiddleware retrieves the extension implementing Middleware based on the middlewareID.
func getMiddleware(extensions map[component.ID]component.Component, middlewareID component.ID) (Middleware, error) {
	if extension, found := extensions[middlewareID]; found {
		if middleware, ok := extension.(Middleware); ok {
			return middleware, nil
		}
		return nil, errNotMiddleware
	}
	return nil, fmt.Errorf("failed to resolve AWS middleware %q: %w", middlewareID, errNotFound)
}

// GetConfigurer retrieves the extension implementing Middleware based on the middlewareID and
// wraps it in a Configurer.
func GetConfigurer(extensions map[component.ID]component.Component, middlewareID component.ID) (*Configurer, error) {
	middleware, err := getMiddleware(extensions, middlewareID)
	if err != nil {
		return nil, err
	}
	return NewConfigurer(middleware.Handlers()), nil
}
