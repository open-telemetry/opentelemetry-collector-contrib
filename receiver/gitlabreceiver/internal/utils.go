// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gitlabreceiver/internal"

import (
	"errors"
	"time"
)

const (
	// GitlabEventTimeFormat is the time format used by GitLab webhook events
	// Format: "2006-01-02 15:04:05 UTC"
	GitlabEventTimeFormat = "2006-01-02 15:04:05 UTC"
)

// ParseGitlabTime parses the time string from a GitLab event.
// It handles two different time formats:
//   - Actual webhook events: "2006-01-02 15:04:05 UTC" (GitlabEventTimeFormat)
//   - Test webhook events: RFC3339 format (e.g., "2025-04-01T18:31:49.624Z")
//
// Returns an error if the time string is empty, "null", or cannot be parsed.
func ParseGitlabTime(t string) (time.Time, error) {
	if t == "" || t == "null" {
		return time.Time{}, errors.New("time is empty")
	}

	// Try actual webhook event format first
	pt, err := time.Parse(GitlabEventTimeFormat, t)
	if err == nil {
		return pt, nil
	}

	// Try RFC3339 format (used in test events)
	pt, err = time.Parse(time.RFC3339, t)
	if err == nil {
		return pt, nil
	}

	return time.Time{}, err
}
