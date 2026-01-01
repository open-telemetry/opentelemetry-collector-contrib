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

//go:build windows

package sidcache

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modadvapi32 = windows.NewLazySystemDLL("advapi32.dll")
	// LsaLookupSids2 is more efficient than LsaLookupSids and returns better format
	procLsaLookupSids2       = modadvapi32.NewProc("LsaLookupSids2")
	procLsaFreeMemory        = modadvapi32.NewProc("LsaFreeMemory")
	procLsaClose             = modadvapi32.NewProc("LsaClose")
	procLsaOpenPolicy        = modadvapi32.NewProc("LsaOpenPolicy")
	procConvertStringSidToSid = modadvapi32.NewProc("ConvertStringSidToSidW")
	procLocalFree            = windows.NewLazySystemDLL("kernel32.dll").NewProc("LocalFree")
)

// LSA constants
const (
	POLICY_LOOKUP_NAMES = 0x00000800
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
	Domains uintptr
}

// LSA_TRUST_INFORMATION structure
type lsaTrustInformation struct {
	Name lsaUnicodeString
	Sid  *windows.SID
}

// LSA_TRANSLATED_NAME structure
type lsaTranslatedName struct {
	Use          uint32
	Name         lsaUnicodeString
	DomainIndex  int32
}

// lookupSID performs a Windows LSA lookup for the given SID string
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
		return nil, fmt.Errorf("ConvertStringSidToSid failed")
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(sid)))

	// Open LSA policy
	var policyHandle windows.Handle
	objAttrs := lsaObjectAttributes{
		Length: uint32(unsafe.Sizeof(lsaObjectAttributes{})),
	}

	status, _, _ := procLsaOpenPolicy.Call(
		0, // Local system
		uintptr(unsafe.Pointer(&objAttrs)),
		POLICY_LOOKUP_NAMES,
		uintptr(unsafe.Pointer(&policyHandle)),
	)
	if status != 0 {
		return nil, fmt.Errorf("LsaOpenPolicy failed with status: %x", status)
	}
	defer procLsaClose.Call(uintptr(policyHandle))

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
		defer procLsaFreeMemory.Call(uintptr(unsafe.Pointer(domains)))
	}
	if names != nil {
		defer procLsaFreeMemory.Call(uintptr(unsafe.Pointer(names)))
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
		domainPtr := unsafe.Pointer(domains.Domains)
		domainInfo := (*lsaTrustInformation)(unsafe.Pointer(
			uintptr(domainPtr) + uintptr(names.DomainIndex)*unsafe.Sizeof(lsaTrustInformation{}),
		))

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

// sidTypeToString converts SID_NAME_USE to a readable string
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
