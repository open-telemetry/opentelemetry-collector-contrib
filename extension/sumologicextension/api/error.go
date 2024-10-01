// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package api // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/sumologicextension/api"

type ErrorResponsePayload struct {
	ID     string  `json:"id"`
	Errors []Error `json:"errors"`
}

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
