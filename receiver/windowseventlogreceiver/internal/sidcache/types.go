// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sidcache

import "time"

// ResolvedSID contains the resolved information for a Windows SID
type ResolvedSID struct {
	// SID is the original Security Identifier (e.g., "S-1-5-21-...")
	SID string

	// AccountName is the fully qualified name (e.g., "DOMAIN\username")
	AccountName string

	// Domain is the domain or authority name (e.g., "ACME", "NT AUTHORITY")
	Domain string

	// Username is the account name without domain (e.g., "jsmith", "SYSTEM")
	Username string

	// AccountType describes the type of account
	// Values: "User", "Group", "Computer", "Alias", "WellKnownGroup", "DeletedAccount", "Invalid", "Unknown"
	AccountType string

	// ResolvedAt is when this SID was resolved (for TTL tracking)
	ResolvedAt time.Time
}

// Cache defines the interface for SID resolution caching
type Cache interface {
	// Resolve looks up a SID and returns its resolved information
	// Returns error if SID cannot be resolved
	Resolve(sid string) (*ResolvedSID, error)

	// Close releases any resources held by the cache
	Close() error

	// Stats returns current cache statistics
	Stats() Stats
}

// Stats contains cache performance metrics
type Stats struct {
	// Hits is the number of successful cache lookups
	Hits uint64

	// Misses is the number of cache misses requiring API calls
	Misses uint64

	// Evictions is the number of cache entries evicted
	Evictions uint64

	// Size is the current number of entries in the cache
	Size int

	// Errors is the number of failed SID resolutions
	Errors uint64
}

// Config contains configuration for the SID cache
type Config struct {
	// Size is the maximum number of entries in the cache (LRU eviction)
	Size int

	// TTL is how long cache entries remain valid
	TTL time.Duration
}
