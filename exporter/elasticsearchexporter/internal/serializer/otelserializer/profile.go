// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelserializer // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer/otelserializer"

import (
	"bytes"
	"encoding/json"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer/otelserializer/serializeprofiles"
)

const (
	AllEventsIndex   = "profiling-events-all"
	StackTraceIndex  = "profiling-stacktraces"
	StackFrameIndex  = "profiling-stackframes"
	ExecutablesIndex = "profiling-executables"
)

// SerializeProfile serializes a profile and calls the `pushData` callback for each generated document.
func SerializeProfile(resource pcommon.Resource, scope pcommon.InstrumentationScope, profile pprofile.Profile, pushData func(*bytes.Buffer, string, string) error) error {
	data, err := serializeprofiles.Transform(resource, scope, profile)
	if err != nil {
		return err
	}

	for _, payload := range data {
		if payload.StackTraceEvent.StackTraceID != "" {
			c, err := toJSON(payload.StackTraceEvent)
			if err != nil {
				return err
			}
			err = pushData(c, "", AllEventsIndex)
			if err != nil {
				return err
			}
		}

		if payload.StackTrace.DocID != "" {
			c, err := toJSON(payload.StackTrace)
			if err != nil {
				return err
			}
			err = pushData(c, payload.StackTrace.DocID, StackTraceIndex)
			if err != nil {
				return err
			}
		}

		for _, stackFrame := range payload.StackFrames {
			c, err := toJSON(stackFrame)
			if err != nil {
				return err
			}
			err = pushData(c, stackFrame.DocID, StackFrameIndex)
			if err != nil {
				return err
			}
		}

		for _, executable := range payload.Executables {
			c, err := toJSON(executable)
			if err != nil {
				return err
			}
			err = pushData(c, executable.DocID, ExecutablesIndex)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func toJSON(d any) (*bytes.Buffer, error) {
	c, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}

	return bytes.NewBuffer(c), nil
}
