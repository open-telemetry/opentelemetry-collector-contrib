// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelserializer // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer/otelserializer"

import (
	"bytes"
	"encoding/json"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"

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
)

// SerializeProfile serializes a profile and calls the `pushData` callback for each generated document.
func (s *Serializer) SerializeProfile(dic pprofile.ProfilesDictionary, resource pcommon.Resource, scope pcommon.InstrumentationScope, profile pprofile.Profile, pushData func(*bytes.Buffer, string, string) error) error {
	err := s.createLRUs()
	if err != nil {
		return err
	}

	pushDataAsJSON := func(data any, id, index string) (err error) {
		c, err := toJSON(data)
		if err != nil {
			return err
		}
		return pushData(c, id, index)
	}

	data, err := serializeprofiles.Transform(dic, resource, scope, profile)
	if err != nil {
		return err
	}

	err = s.knownTraces.WithLock(func(tracesSet lru.LockedLRUSet) error {
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
		}

		return nil
	})
	if err != nil {
		return err
	}

	err = s.knownFrames.WithLock(func(framesSet lru.LockedLRUSet) error {
		for _, payload := range data {
			for _, stackFrame := range payload.StackFrames {
				if !framesSet.CheckAndAdd(stackFrame.DocID) {
					if err = pushDataAsJSON(stackFrame, stackFrame.DocID, StackFrameIndex); err != nil {
						return err
					}
				}
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	err = s.knownExecutables.WithLock(func(executablesSet lru.LockedLRUSet) error {
		for _, payload := range data {
			for _, executable := range payload.Executables {
				if !executablesSet.CheckAndAdd(executable.DocID) {
					if err = pushDataAsJSON(executable, executable.DocID, ExecutablesIndex); err != nil {
						return err
					}
				}
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	err = s.knownUnsymbolizedFrames.WithLock(func(unsymbolizedFramesSet lru.LockedLRUSet) error {
		for _, payload := range data {
			for _, frame := range payload.UnsymbolizedLeafFrames {
				if !unsymbolizedFramesSet.CheckAndAdd(frame.DocID) {
					if err = pushDataAsJSON(frame, frame.DocID, LeafFramesSymQueueIndex); err != nil {
						return err
					}
				}
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	return s.knownUnsymbolizedExecutables.WithLock(func(unsymbolizedExecutablesSet lru.LockedLRUSet) error {
		for _, payload := range data {
			for _, executable := range payload.UnsymbolizedExecutables {
				if !unsymbolizedExecutablesSet.CheckAndAdd(executable.DocID) {
					if err = pushDataAsJSON(executable, executable.DocID, ExecutablesSymQueueIndex); err != nil {
						return err
					}
				}
			}
		}

		return nil
	})
}

func toJSON(d any) (*bytes.Buffer, error) {
	c, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}

	return bytes.NewBuffer(c), nil
}

func (s *Serializer) createLRUs() error {
	s.loadLRUsOnce.Do(func() {
		var err error

		// Create LRUs with MinILMRolloverTime as lifetime to avoid losing data by ILM roll-over.
		s.knownTraces, err = lru.NewLRUSet(knownTracesCacheSize, minILMRolloverTime)
		if err != nil {
			s.lruErr = fmt.Errorf("failed to create traces LRU: %w", err)
			return
		}

		s.knownFrames, err = lru.NewLRUSet(knownFramesCacheSize, minILMRolloverTime)
		if err != nil {
			s.lruErr = fmt.Errorf("failed to create frames LRU: %w", err)
			return
		}

		s.knownExecutables, err = lru.NewLRUSet(knownExecutablesCacheSize, minILMRolloverTime)
		if err != nil {
			s.lruErr = fmt.Errorf("failed to create executables LRU: %w", err)
			return
		}

		s.knownUnsymbolizedFrames, err = lru.NewLRUSet(knownUnsymbolizedFramesCacheSize, minILMRolloverTime)
		if err != nil {
			s.lruErr = fmt.Errorf("failed to create unsymbolized frames LRU: %w", err)
			return
		}

		s.knownUnsymbolizedExecutables, err = lru.NewLRUSet(knownUnsymbolizedExecutablesCacheSize, minILMRolloverTime)
		if err != nil {
			s.lruErr = fmt.Errorf("failed to create unsymbolized executables LRU: %w", err)
			return
		}
	})

	return s.lruErr
}
