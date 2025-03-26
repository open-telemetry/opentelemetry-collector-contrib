// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelserializer // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer/otelserializer"

import (
	"bytes"
	"encoding/json"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"

	"github.com/cespare/xxhash"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/lru"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer/otelserializer/serializeprofiles"
)

const (
	AllEventsIndex   = "profiling-events-all"
	StackTraceIndex  = "profiling-stacktraces"
	StackFrameIndex  = "profiling-stackframes"
	ExecutablesIndex = "profiling-executables"

	ExecutablesSymQueueIndex = "profiling-sq-executables"
	LeafFramesSymQueueIndex  = "profiling-sq-leafframes"

	KiB = 1024
	MiB = 1024 * KiB

	knownExecutablesCacheSize = 128 * KiB
	knownFramesCacheSize      = 128 * KiB
	knownTracesCacheSize      = 128 * KiB

	minILMRolloverTime = 3 * time.Hour
)

var stringHashFn = func(s string) uint32 {
	return uint32(xxhash.Sum64String(s))
}

// SerializeProfile serializes a profile and calls the `pushData` callback for each generated document.
func (s *Serializer) SerializeProfile(resource pcommon.Resource, scope pcommon.InstrumentationScope, profile pprofile.Profile, pushData func(*bytes.Buffer, string, string) error) error {
	pushDataAsJSON := func(data any, id, index string) error {
		c, err := toJSON(data)
		if err != nil {
			return err
		}
		return pushData(c, id, index)
	}

	data, err := serializeprofiles.Transform(resource, scope, profile)
	if err != nil {
		return err
	}
	return s.knownTraces.WithLock(func(tracesSet lru.LockedLRUSet[string]) error {
		return s.knownFrames.WithLock(func(framesSet lru.LockedLRUSet[string]) error {
			return s.knownExecutables.WithLock(func(executablesSet lru.LockedLRUSet[string]) error {
				for _, payload := range data {
					event := payload.StackTraceEvent

					if event.StackTraceID != "" {
						if err = pushDataAsJSON(event, "", AllEventsIndex); err != nil {
							return err
						}
						if err = serializeprofiles.IndexDownsampledEvent(event, pushDataAsJSON); err != nil {
							return err
						}
					}

					if payload.StackTrace.DocID != "" {
						if !tracesSet.CheckAndAdd(payload.StackTrace.DocID) {
							if err = pushDataAsJSON(payload.StackTrace, payload.StackTrace.DocID, StackTraceIndex); err != nil {
								return err
							}
						}
					}

					for _, stackFrame := range payload.StackFrames {
						if !framesSet.CheckAndAdd(stackFrame.DocID) {
							if err = pushDataAsJSON(stackFrame, stackFrame.DocID, StackFrameIndex); err != nil {
								return err
							}
						}
					}

					for _, executable := range payload.Executables {
						if !executablesSet.CheckAndAdd(executable.DocID) {
							if err = pushDataAsJSON(executable, executable.DocID, ExecutablesIndex); err != nil {
								return err
							}
						}
					}

					for _, frame := range payload.UnsymbolizedLeafFrames {
						if err = pushDataAsJSON(frame, frame.DocID, LeafFramesSymQueueIndex); err != nil {
							return err
						}
					}

					for _, executable := range payload.UnsymbolizedExecutables {
						if err = pushDataAsJSON(executable, executable.DocID, ExecutablesSymQueueIndex); err != nil {
							return err
						}
					}
				}

				return nil
			})
		})
	})
}

func toJSON(d any) (*bytes.Buffer, error) {
	c, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}

	return bytes.NewBuffer(c), nil
}
