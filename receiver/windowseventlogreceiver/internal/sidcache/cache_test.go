// Copyright observIQ, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sidcache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectSize  int
		expectTTL   time.Duration
		expectError bool
	}{
		{
			name: "valid config",
			config: Config{
				Size: 100,
				TTL:  5 * time.Minute,
			},
			expectSize: 100,
			expectTTL:  5 * time.Minute,
		},
		{
			name:       "zero config uses defaults",
			config:     Config{},
			expectSize: DefaultCacheSize,
			expectTTL:  DefaultCacheTTL,
		},
		{
			name: "negative size uses default",
			config: Config{
				Size: -1,
				TTL:  5 * time.Minute,
			},
			expectSize: DefaultCacheSize,
			expectTTL:  5 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache, err := New(tt.config)
			if tt.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, cache)

			// Verify cache is functional by resolving a well-known SID
			resolved, err := cache.Resolve("S-1-5-18")
			require.NoError(t, err)
			require.NotNil(t, resolved)
			require.Equal(t, "NT AUTHORITY\\SYSTEM", resolved.AccountName)

			// Clean up
			require.NoError(t, cache.Close())
		})
	}
}

func TestWellKnownSIDs(t *testing.T) {
	cache, err := New(Config{})
	require.NoError(t, err)
	defer cache.Close()

	tests := []struct {
		name            string
		sid             string
		expectName      string
		expectDomain    string
		expectUsername  string
		expectType      string
		expectError     bool
	}{
		{
			name:           "SYSTEM",
			sid:            "S-1-5-18",
			expectName:     "NT AUTHORITY\\SYSTEM",
			expectDomain:   "NT AUTHORITY",
			expectUsername: "SYSTEM",
			expectType:     "WellKnownGroup",
		},
		{
			name:           "LOCAL SERVICE",
			sid:            "S-1-5-19",
			expectName:     "NT AUTHORITY\\LOCAL SERVICE",
			expectDomain:   "NT AUTHORITY",
			expectUsername: "LOCAL SERVICE",
			expectType:     "WellKnownGroup",
		},
		{
			name:           "NETWORK SERVICE",
			sid:            "S-1-5-20",
			expectName:     "NT AUTHORITY\\NETWORK SERVICE",
			expectDomain:   "NT AUTHORITY",
			expectUsername: "NETWORK SERVICE",
			expectType:     "WellKnownGroup",
		},
		{
			name:           "Administrators",
			sid:            "S-1-5-32-544",
			expectName:     "BUILTIN\\Administrators",
			expectDomain:   "BUILTIN",
			expectUsername: "Administrators",
			expectType:     "Alias",
		},
		{
			name:           "Everyone",
			sid:            "S-1-1-0",
			expectName:     "Everyone",
			expectDomain:   "",
			expectUsername: "Everyone",
			expectType:     "WellKnownGroup",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved, err := cache.Resolve(tt.sid)

			if tt.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, resolved)
			require.Equal(t, tt.sid, resolved.SID)
			require.Equal(t, tt.expectName, resolved.AccountName)
			require.Equal(t, tt.expectDomain, resolved.Domain)
			require.Equal(t, tt.expectUsername, resolved.Username)
			require.Equal(t, tt.expectType, resolved.AccountType)
		})
	}

	// Verify well-known SIDs are counted as hits
	stats := cache.Stats()
	require.Equal(t, uint64(5), stats.Hits)
	require.Equal(t, uint64(0), stats.Misses)
}

func TestInvalidSIDFormat(t *testing.T) {
	cache, err := New(Config{})
	require.NoError(t, err)
	defer cache.Close()

	tests := []struct {
		name string
		sid  string
	}{
		{"empty string", ""},
		{"not a SID", "not-a-sid"},
		{"missing parts", "S-1"},
		{"invalid format", "X-1-5-18"},
		{"malformed", "S-X-5-18"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved, err := cache.Resolve(tt.sid)
			require.Error(t, err)
			require.Nil(t, resolved)
		})
	}

	stats := cache.Stats()
	require.Equal(t, uint64(len(tests)), stats.Errors)
}

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

func TestCacheStats(t *testing.T) {
	cache, err := New(Config{Size: 2, TTL: 1 * time.Hour})
	require.NoError(t, err)
	defer cache.Close()

	// Resolve a well-known SID (hit)
	_, err = cache.Resolve("S-1-5-18")
	require.NoError(t, err)

	stats := cache.Stats()
	require.Equal(t, uint64(1), stats.Hits)
	require.Equal(t, uint64(0), stats.Misses)
	require.Equal(t, uint64(0), stats.Errors)

	// Invalid SID (error)
	_, err = cache.Resolve("invalid")
	require.Error(t, err)

	stats = cache.Stats()
	require.Equal(t, uint64(1), stats.Hits)
	require.Equal(t, uint64(0), stats.Misses)
	require.Equal(t, uint64(1), stats.Errors)
}

func TestCacheClose(t *testing.T) {
	cache, err := New(Config{})
	require.NoError(t, err)

	// Resolve some well-known SIDs (these don't get cached, but verify cache works)
	_, err1 := cache.Resolve("S-1-5-18")
	_, err2 := cache.Resolve("S-1-5-19")
	require.NoError(t, err1)
	require.NoError(t, err2)

	// Close should succeed
	err = cache.Close()
	require.NoError(t, err)

	// Stats should show empty cache after close
	stats := cache.Stats()
	require.Equal(t, 0, stats.Size)
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
