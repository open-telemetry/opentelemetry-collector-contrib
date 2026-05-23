// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package sidcache // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowseventlogreceiver/internal/sidcache"

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	lru "github.com/hashicorp/golang-lru/v2"
	"golang.org/x/sys/windows"
)

var (
	modadvapi32 = windows.NewLazySystemDLL("advapi32.dll")
	// LsaLookupSids2 is more efficient than LsaLookupSids and returns better format
	procLsaLookupSids2        = modadvapi32.NewProc("LsaLookupSids2")
	procLsaFreeMemory         = modadvapi32.NewProc("LsaFreeMemory")
	procLsaClose              = modadvapi32.NewProc("LsaClose")
	procLsaOpenPolicy         = modadvapi32.NewProc("LsaOpenPolicy")
	procConvertStringSidToSid = modadvapi32.NewProc("ConvertStringSidToSidW")
	procLocalFree             = windows.NewLazySystemDLL("kernel32.dll").NewProc("LocalFree")
)

// LSA constants
const (
	policyLookupNames = 0x00000800
)

// SID_NAME_USE enumeration
// https://learn.microsoft.com/en-us/windows/win32/api/winnt/ne-winnt-sid_name_use
const (
	SidTypeUser           = 1
	SidTypeGroup          = 2
	SidTypeDomain         = 3
	SidTypeAlias          = 4
	SidTypeWellKnownGroup = 5
	SidTypeDeletedAccount = 6
	SidTypeInvalid        = 7
	SidTypeUnknown        = 8
	SidTypeComputer       = 9
)

// LSA_UNICODE_STRING structure
type lsaUnicodeString struct {
	Length        uint16
	MaximumLength uint16
	Buffer        *uint16
}

// LSA_OBJECT_ATTRIBUTES structure
type lsaObjectAttributes struct {
	Length                   uint32
	RootDirectory            windows.Handle
	ObjectName               *lsaUnicodeString
	Attributes               uint32
	SecurityDescriptor       uintptr
	SecurityQualityOfService uintptr
}

// LSA_REFERENCED_DOMAIN_LIST structure
type lsaReferencedDomainList struct {
	Entries uint32
	Domains unsafe.Pointer
}

// LSA_TRUST_INFORMATION structure
type lsaTrustInformation struct {
	Name lsaUnicodeString
	Sid  *windows.SID
}

// LSA_TRANSLATED_NAME structure
type lsaTranslatedName struct {
	Use         uint32
	Name        lsaUnicodeString
	DomainIndex int32
}

// cacheEntry wraps a ResolvedSID with TTL information
type cacheEntry struct {
	resolved  *ResolvedSID
	expiresAt time.Time
}

// concurrentSIDCache implements the Cache interface with LRU eviction and TTL expiration.
// It is safe for concurrent use by multiple goroutines.
type concurrentSIDCache struct {
	config Config
	lru    *lru.Cache[string, *cacheEntry]

	// Statistics (using atomic for lock-free thread safety)
	hits      atomic.Uint64
	misses    atomic.Uint64
	evictions atomic.Uint64
	errors    atomic.Uint64

	mu sync.RWMutex
}

// New creates a new SID cache with the given configuration.
// The returned Cache is safe for concurrent use.
func New(config Config) (Cache, error) {
	// Validate and set defaults
	if config.Size == 0 {
		config.Size = DefaultCacheSize
	}
	if config.TTL <= 0 {
		config.TTL = DefaultCacheTTL
	}

	c := &concurrentSIDCache{
		config: config,
	}

	// Create LRU cache with eviction callback
	lruCache, err := lru.NewWithEvict(int(config.Size), func(_ string, _ *cacheEntry) {
		c.evictions.Add(1)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create LRU cache: %w", err)
	}

	c.lru = lruCache
	return c, nil
}

// Resolve looks up a SID and returns its resolved information.
func (c *concurrentSIDCache) Resolve(sid string) (*ResolvedSID, error) {
	// Validate SID format
	if !isSIDFormat(sid) {
		c.errors.Add(1)
		return nil, fmt.Errorf("invalid SID format: %s", sid)
	}

	// Check well-known SIDs first (never cached, always available)
	if resolved, ok := isWellKnownSID(sid); ok {
		c.hits.Add(1)
		return resolved, nil
	}

	// Check cache
	c.mu.RLock()
	entry, found := c.lru.Get(sid)
	c.mu.RUnlock()

	if found {
		// Check if entry has expired
		if time.Now().Before(entry.expiresAt) {
			c.hits.Add(1)
			return entry.resolved, nil
		}
		// Entry expired, remove it
		c.mu.Lock()
		c.lru.Remove(sid)
		c.mu.Unlock()
	}

	// Cache miss - resolve via Windows API
	c.misses.Add(1)
	resolved, err := lookupSID(sid)
	if err != nil {
		c.errors.Add(1)
		return nil, fmt.Errorf("failed to lookup SID %s: %w", sid, err)
	}

	// Add to cache with TTL
	c.mu.Lock()
	c.lru.Add(sid, &cacheEntry{
		resolved:  resolved,
		expiresAt: time.Now().Add(c.config.TTL),
	})
	c.mu.Unlock()

	return resolved, nil
}

// Close releases resources held by the cache.
// Purge is called explicitly rather than relying on GC to provide deterministic
// cleanup of cached entries, which is useful for tests and follows the Cache
// interface contract.
func (c *concurrentSIDCache) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.lru.Purge()
	return nil
}

// Stats returns current cache statistics.
func (c *concurrentSIDCache) Stats() Stats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return Stats{
		Hits:      c.hits.Load(),
		Misses:    c.misses.Load(),
		Evictions: c.evictions.Load(),
		Size:      c.lru.Len(),
		Errors:    c.errors.Load(),
	}
}

// lookupSID performs a Windows LSA lookup for the given SID string.
func lookupSID(sidString string) (*ResolvedSID, error) {
	// Convert string SID to binary SID
	var sid *windows.SID
	sidPtr, err := syscall.UTF16PtrFromString(sidString)
	if err != nil {
		return nil, fmt.Errorf("failed to convert SID string: %w", err)
	}

	ret, _, _ := procConvertStringSidToSid.Call(
		uintptr(unsafe.Pointer(sidPtr)),
		uintptr(unsafe.Pointer(&sid)),
	)
	if ret == 0 {
		return nil, errors.New("ConvertStringSidToSid failed")
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(sid))) //nolint:errcheck // cleanup: no action on error

	// Open LSA policy
	var policyHandle windows.Handle
	objAttrs := lsaObjectAttributes{
		Length: uint32(unsafe.Sizeof(lsaObjectAttributes{})),
	}

	status, _, _ := procLsaOpenPolicy.Call(
		0, // Local system
		uintptr(unsafe.Pointer(&objAttrs)),
		policyLookupNames,
		uintptr(unsafe.Pointer(&policyHandle)),
	)
	if status != 0 {
		return nil, fmt.Errorf("LsaOpenPolicy failed with status: %x", status)
	}
	defer procLsaClose.Call(uintptr(policyHandle)) //nolint:errcheck // cleanup: no action on error

	// Prepare SID array for LsaLookupSids2
	sidArray := []*windows.SID{sid}

	var domains *lsaReferencedDomainList
	var names *lsaTranslatedName

	// Call LsaLookupSids2
	status, _, _ = procLsaLookupSids2.Call(
		uintptr(policyHandle),
		0, // No flags
		1, // One SID
		uintptr(unsafe.Pointer(&sidArray[0])),
		uintptr(unsafe.Pointer(&domains)),
		uintptr(unsafe.Pointer(&names)),
	)

	// Clean up LSA memory
	if domains != nil {
		defer procLsaFreeMemory.Call(uintptr(unsafe.Pointer(domains))) //nolint:errcheck // cleanup: no action on error
	}
	if names != nil {
		defer procLsaFreeMemory.Call(uintptr(unsafe.Pointer(names))) //nolint:errcheck // cleanup: no action on error
	}

	// status == 0 means success, 0xC0000073 (STATUS_NONE_MAPPED) means SID not found
	if status == 0xC0000073 {
		return nil, fmt.Errorf("SID not found: %s", sidString)
	}
	if status != 0 && status != 0xC0000107 { // 0xC0000107 is STATUS_SOME_NOT_MAPPED (partial success)
		return nil, fmt.Errorf("LsaLookupSids2 failed with status: %x", status)
	}

	// Extract the translated name
	nameUTF16 := unsafe.Slice(names.Name.Buffer, names.Name.Length/2)
	username := syscall.UTF16ToString(nameUTF16)

	// Extract domain name if available
	domain := ""
	if names.DomainIndex >= 0 && domains != nil && uint32(names.DomainIndex) < domains.Entries {
		// Get the domain from the domain list
		domainInfo := (*lsaTrustInformation)(unsafe.Add(domains.Domains, uintptr(names.DomainIndex)*unsafe.Sizeof(lsaTrustInformation{})))

		if domainInfo.Name.Buffer != nil {
			domainUTF16 := unsafe.Slice(domainInfo.Name.Buffer, domainInfo.Name.Length/2)
			domain = syscall.UTF16ToString(domainUTF16)
		}
	}

	// Build the account name
	accountName := username
	if domain != "" {
		accountName = domain + "\\" + username
	}

	// Map SID type to string
	accountType := sidTypeToString(names.Use)

	return &ResolvedSID{
		SID:         sidString,
		AccountName: accountName,
		Domain:      domain,
		Username:    username,
		AccountType: accountType,
		ResolvedAt:  time.Now(),
	}, nil
}

// sidTypeToString converts SID_NAME_USE to a readable string.
func sidTypeToString(sidType uint32) string {
	switch sidType {
	case SidTypeUser:
		return "User"
	case SidTypeGroup:
		return "Group"
	case SidTypeDomain:
		return "Domain"
	case SidTypeAlias:
		return "Alias"
	case SidTypeWellKnownGroup:
		return "WellKnownGroup"
	case SidTypeDeletedAccount:
		return "DeletedAccount"
	case SidTypeInvalid:
		return "Invalid"
	case SidTypeComputer:
		return "Computer"
	default:
		return "Unknown"
	}
}
