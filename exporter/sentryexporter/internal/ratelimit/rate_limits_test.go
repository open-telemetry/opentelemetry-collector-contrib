// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ratelimit

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

var now = time.Date(2008, 5, 12, 16, 26, 19, 0, time.UTC)

func TestParseXSentryRateLimits(t *testing.T) {
	tests := []struct {
		input      string
		wantLimits Map
	}{
		// Empty rate limits == nothing is rate-limited
		{"", Map{}},
		{",", Map{}},
		{",,,,", Map{}},
		{",  ,   ,     ,", Map{}},
		{":", Map{}},
		{":::", Map{}},
		{"::,,:,", Map{}},
		{":,:;;;:", Map{}},

		{
			"1",
			Map{CategoryAll: Deadline(now.Add(1 * time.Second))},
		},
		{
			"2::ignored_scope:ignored_reason",
			Map{CategoryAll: Deadline(now.Add(2 * time.Second))},
		},
		{
			"3::ignored_scope:ignored_reason",
			Map{CategoryAll: Deadline(now.Add(3 * time.Second))},
		},

		{
			"4:log_item",
			Map{CategoryLog: Deadline(now.Add(4 * time.Second))},
		},
		{
			"5:log_item;transaction",
			Map{
				CategoryLog:         Deadline(now.Add(5 * time.Second)),
				CategoryTransaction: Deadline(now.Add(5 * time.Second)),
			},
		},
		{
			"6:log_item, 7:transaction",
			Map{
				CategoryLog:         Deadline(now.Add(6 * time.Second)),
				CategoryTransaction: Deadline(now.Add(7 * time.Second)),
			},
		},
		{
			// ignore unknown categories
			"8:log_item;default;unknown",
			Map{
				CategoryLog: Deadline(now.Add(8 * time.Second)),
			},
		},
		{
			"30:log_item:scope1, 20:log_item:scope2, 40:log_item",
			Map{CategoryLog: Deadline(now.Add(40 * time.Second))},
		},
		{
			"30:log_item:scope1, 20:log_item:scope2, 40::",
			Map{
				CategoryAll: Deadline(now.Add(40 * time.Second)),
				CategoryLog: Deadline(now.Add(30 * time.Second)),
			},
		},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%q", tt.input), func(t *testing.T) {
			got, want := parseXSentryRateLimits(tt.input, now), tt.wantLimits
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}

func TestParseXSRLRetryAfterValidInput(t *testing.T) {
	// https://github.com/getsentry/relay/blob/0424a2e017d193a93918053c90cdae9472d164bf/relay-quotas/src/rate_limit.rs#L88-L96
	tests := []struct {
		input string
		want  Deadline
	}{
		// Integers are the common case
		{"0", Deadline(now)},
		{"1", Deadline(now.Add(1 * time.Second))},
		{"60", Deadline(now.Add(1 * time.Minute))},

		// Any fractional increment round up to the next full second
		// (replicating implementation in getsentry/relay)
		{"3.1", Deadline(now.Add(4 * time.Second))},
		{"3.5", Deadline(now.Add(4 * time.Second))},
		{"3.9", Deadline(now.Add(4 * time.Second))},

		// Overflows are treated like zero
		{"100000000000000000", Deadline(now)},

		// Negative numbers are treated like zero
		{"-Inf", Deadline(now)},
		{"-0", Deadline(now)},
		{"-1", Deadline(now)},

		// Special floats are treated like zero
		{"Inf", Deadline(now)},
		{"NaN", Deadline(now)},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%q", tt.input), func(t *testing.T) {
			d, err := parseXSRLRetryAfter(tt.input, now)
			if err != nil {
				t.Fatalf("got %v, want nil", err)
			}
			got, want := time.Time(d), time.Time(tt.want)
			if !got.Equal(want) {
				t.Errorf("got %v, want %v", got, want)
			}
		})
	}
}

func TestParseXSRLRetryAfterInvalidInput(t *testing.T) {
	// https://github.com/getsentry/relay/blob/0424a2e017d193a93918053c90cdae9472d164bf/relay-quotas/src/rate_limit.rs#L88-L96
	tests := []struct {
		input string
	}{
		{""},
		{"invalid"},
		{" 2 "},
		{"6 0"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := parseXSRLRetryAfter(tt.input, now)
			if err == nil {
				t.Fatalf("got %v, want nil", err)
			}
			t.Log(err)
		})
	}
}
