// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver"

import "sync"

type partitionPauseReason uint8

const (
	partitionPauseBackpressure partitionPauseReason = 1 << iota
	partitionPauseRewind
	partitionPauseProcessingError
)

// partitionPauseState tracks every receiver-owned reason for pausing one
// partition. franz-go stores only one paused bit, so the receiver must not
// resume fetching until all of its reasons have cleared.
type partitionPauseState struct {
	mu      sync.Mutex
	reasons partitionPauseReason
}

// pause adds a reason and pauses franz-go when the first reason appears.
func (s *partitionPauseState) pause(
	controller partitionFetchController,
	partition map[string][]int32,
	reason partitionPauseReason,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	wasPaused := s.reasons != 0
	s.reasons |= reason
	if !wasPaused {
		controller.PauseFetchPartitions(partition)
	}
}

// resume clears a reason and resumes franz-go after the final reason clears.
func (s *partitionPauseState) resume(
	controller partitionFetchController,
	partition map[string][]int32,
	reason partitionPauseReason,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	wasPaused := s.reasons != 0
	s.reasons &^= reason
	if wasPaused && s.reasons == 0 {
		controller.ResumeFetchPartitions(partition)
	}
}
