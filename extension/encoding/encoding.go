// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package encoding // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
)

var errNoExtensionFound = "no extension found: %s"
var errNotAnEncodingExtension = "extension %q doesn't implement %T"

/*
Receivers wanting to Unmarshal can use the GetEncoding like following:

	unmarshaler, err := GetEncoding[(pmetric|ptrace|plog).Unmarshaler](host.GetExtensions(), id)

Exporters wanting to Marshal can use the GetEncoding like following:

	marshaler, err := GetEncoding[(pmetric|ptrace|plog).Marshaler](host.GetExtensions(), id)
*/
func GetEncoding[T any](extensions map[component.ID]component.Component, componentID component.ID) (T, error) {
	var val T
	if ext, ok := extensions[componentID]; ok {
		unmarshaler, ok := ext.(T)
		if !ok {
			return val, fmt.Errorf(errNotAnEncodingExtension, componentID, &val)
		}
		return unmarshaler, nil
	}
	return val, fmt.Errorf(errNoExtensionFound, componentID)
}
