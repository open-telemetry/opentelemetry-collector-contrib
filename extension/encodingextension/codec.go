// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package encodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encodingextension"

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// Metric codec marshals and unmarshals metric pdata.
type Metric interface {
	pmetric.Marshaler
	pmetric.Unmarshaler
}

// Log codec marshals and unmarshals log pdata.
type Log interface {
	plog.Marshaler
	plog.Unmarshaler
}

// Trace codec marshals and unmarshals trace pdata.
type Trace interface {
	ptrace.Marshaler
	ptrace.Unmarshaler
}

// Extension is the interface that encoding extensions must implement.
type Extension interface {
	extension.Extension
	// GetLogCodec returns a log codec for use by the specified component.
	GetLogCodec() (Log, error)

	// GetMetricCodec returns a metric codec for use by the specified component.
	GetMetricCodec() (Metric, error)

	// GetTraceCodec returns a trace codec for use by the specified component.
	GetTraceCodec() (Trace, error)
}

// GetLogCodec returns a log codec for use by the specified component.
func GetLogCodec(host component.Host, encodingExtensionID *component.ID) (Log, error) {
	if encodingExtensionID == nil {
		return nil, fmt.Errorf("encoding extension cannot be nil")
	}

	extension, ok := host.GetExtensions()[*encodingExtensionID]
	if !ok {
		return nil, fmt.Errorf("encoding extension '%s' not found", encodingExtensionID)
	}

	codecExtension, ok := extension.(Extension)
	if !ok {
		return nil, fmt.Errorf("non-encoding extension '%s' found", encodingExtensionID)
	}

	return codecExtension.GetLogCodec()
}

// GetMetricCodec returns a metric codec for use by the specified component.
func GetMetricCodec(host component.Host, encodingExtensionID *component.ID) (Metric, error) {
	if encodingExtensionID == nil {
		return nil, fmt.Errorf("encoding extension cannot be nil")
	}

	extension, ok := host.GetExtensions()[*encodingExtensionID]
	if !ok {
		return nil, fmt.Errorf("encoding extension '%s' not found", encodingExtensionID)
	}

	codecExtension, ok := extension.(Extension)
	if !ok {
		return nil, fmt.Errorf("non-encoding extension '%s' found", encodingExtensionID)
	}

	return codecExtension.GetMetricCodec()
}

// GetTraceCodec returns a trace codec for use by the specified component.
func GetTraceCodec(host component.Host, encodingExtensionID *component.ID) (Trace, error) {
	if encodingExtensionID == nil {
		return nil, fmt.Errorf("encoding extension cannot be nil")
	}

	extension, ok := host.GetExtensions()[*encodingExtensionID]
	if !ok {
		return nil, fmt.Errorf("encoding extension '%s' not found", encodingExtensionID)
	}

	codecExtension, ok := extension.(Extension)
	if !ok {
		return nil, fmt.Errorf("non-encoding extension '%s' found", encodingExtensionID)
	}

	return codecExtension.GetTraceCodec()
}
