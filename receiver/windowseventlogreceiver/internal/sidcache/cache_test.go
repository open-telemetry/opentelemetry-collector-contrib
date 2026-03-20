// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sidcache

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsSIDField(t *testing.T) {
	tests := []struct {
		fieldName string
		expect    bool
	}{
		{"SubjectUserSid", true},
		{"TargetUserSid", true},
		{"UserSid", true},
		{"Sid", true},
		{"UserID", true},
		{"UserName", false},
		{"EventID", false},
		{"Si", false},
		{"", false},
		{"AccountName", false},
	}

	for _, tt := range tests {
		t.Run(tt.fieldName, func(t *testing.T) {
			result := IsSIDField(tt.fieldName)
			require.Equal(t, tt.expect, result)
		})
	}
}

func TestIsSIDFormat(t *testing.T) {
	tests := []struct {
		sid    string
		expect bool
	}{
		{"S-1-5-18", true},
		{"S-1-5-32-544", true},
		{"S-1-5-21-3623811015-3361044348-30300820-1013", true},
		{"S-1-0-0", true},
		{"", false},
		{"S-1", false},
		{"S-1-5", false},
		{"X-1-5-18", false},
		{"S-X-5-18", false},
		{"not-a-sid", false},
	}

	for _, tt := range tests {
		t.Run(tt.sid, func(t *testing.T) {
			result := isSIDFormat(tt.sid)
			require.Equal(t, tt.expect, result)
		})
	}
}
