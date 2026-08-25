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
	AllEventsIndex   = "profiling-events-all.otel-default"
	StackTraceIndex  = "profiling-stacktraces.otel-default"
	StackFrameIndex  = "profiling-stackframes.otel-default"
	ExecutablesIndex = "profiling-executables"

	HostsMetadataIndex = "profiling-hosts"
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
		for i := range data {
			payload := &data[i]
			event := payload.StackTraceEvent

			if event.StackTraceID != "" {
				err = pushDataAsJSON(event, "", AllEventsIndex)
				if err != nil {
					return err
				}
				err = serializeprofiles.IndexDownsampledEvent(event, ".otel-default", pushDataAsJSON)
				if err != nil {
					return err
				}
			}

			if payload.StackTrace.DocID != "" {
				if !tracesSet.CheckAndAdd(payload.StackTrace.DocID) {
					err = pushDataAsJSON(payload.StackTrace, payload.StackTrace.DocID, StackTraceIndex)
					if err != nil {
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
		for i := range data {
			payload := &data[i]
			for j := range payload.StackFrames {
				stackFrame := &payload.StackFrames[j]
				if !framesSet.CheckAndAdd(stackFrame.DocID) {
					err = pushDataAsJSON(stackFrame, stackFrame.DocID, StackFrameIndex)
					if err != nil {
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
		for i := range data {
			payload := &data[i]
			for _, executable := range payload.Executables {
				if !executablesSet.CheckAndAdd(executable.DocID) {
					err = pushDataAsJSON(executable, executable.DocID, ExecutablesIndex)
					if err != nil {
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

	err = s.knownHosts.WithLock(func(hostMetadata lru.LockedLRUSet) error {
		for i := range data {
			payload := &data[i]
			hostID := payload.ResourceAttrs.HostID
			if hostID == "" {
				continue
			}

			if !hostMetadata.CheckAndAdd(hostID) {
				err = pushDataAsJSON(payload.ResourceAttrs, "", HostsMetadataIndex)
				if err != nil {
					return err
				}
			}
		}
		return nil
	})
	return err
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

		s.knownHosts, err = lru.NewLRUSet(knownHostsCacheSize, minILMRolloverTime)
		if err != nil {
			s.lruErr = fmt.Errorf("failed to create hosts LRU: %w", err)
			return
		}
	})

	return s.lruErr
}
