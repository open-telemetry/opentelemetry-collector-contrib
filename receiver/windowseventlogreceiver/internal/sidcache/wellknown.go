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

// wellKnownSIDs is a map of well-known Windows SIDs to their resolved information
// These SIDs are constant and don't require API lookups
// Reference: https://learn.microsoft.com/en-us/windows-server/identity/ad-ds/manage/understand-security-identifiers
var wellKnownSIDs = map[string]*ResolvedSID{
	// Null Authority
	"S-1-0-0": {
		SID:         "S-1-0-0",
		AccountName: "NULL SID",
		Domain:      "",
		Username:    "NULL SID",
		AccountType: "WellKnownGroup",
	},

	// World Authority
	"S-1-1-0": {
		SID:         "S-1-1-0",
		AccountName: "Everyone",
		Domain:      "",
		Username:    "Everyone",
		AccountType: "WellKnownGroup",
	},

	// Local Authority
	"S-1-2-0": {
		SID:         "S-1-2-0",
		AccountName: "LOCAL",
		Domain:      "",
		Username:    "LOCAL",
		AccountType: "WellKnownGroup",
	},
	"S-1-2-1": {
		SID:         "S-1-2-1",
		AccountName: "CONSOLE LOGON",
		Domain:      "",
		Username:    "CONSOLE LOGON",
		AccountType: "WellKnownGroup",
	},

	// Creator Authority
	"S-1-3-0": {
		SID:         "S-1-3-0",
		AccountName: "CREATOR OWNER",
		Domain:      "",
		Username:    "CREATOR OWNER",
		AccountType: "WellKnownGroup",
	},
	"S-1-3-1": {
		SID:         "S-1-3-1",
		AccountName: "CREATOR GROUP",
		Domain:      "",
		Username:    "CREATOR GROUP",
		AccountType: "WellKnownGroup",
	},
	"S-1-3-4": {
		SID:         "S-1-3-4",
		AccountName: "OWNER RIGHTS",
		Domain:      "",
		Username:    "OWNER RIGHTS",
		AccountType: "WellKnownGroup",
	},

	// NT Authority - Most Common
	"S-1-5-18": {
		SID:         "S-1-5-18",
		AccountName: "NT AUTHORITY\\SYSTEM",
		Domain:      "NT AUTHORITY",
		Username:    "SYSTEM",
		AccountType: "WellKnownGroup",
	},
	"S-1-5-19": {
		SID:         "S-1-5-19",
		AccountName: "NT AUTHORITY\\LOCAL SERVICE",
		Domain:      "NT AUTHORITY",
		Username:    "LOCAL SERVICE",
		AccountType: "WellKnownGroup",
	},
	"S-1-5-20": {
		SID:         "S-1-5-20",
		AccountName: "NT AUTHORITY\\NETWORK SERVICE",
		Domain:      "NT AUTHORITY",
		Username:    "NETWORK SERVICE",
		AccountType: "WellKnownGroup",
	},

	// Built-in Domain Groups
	"S-1-5-32-544": {
		SID:         "S-1-5-32-544",
		AccountName: "BUILTIN\\Administrators",
		Domain:      "BUILTIN",
		Username:    "Administrators",
		AccountType: "Alias",
	},
	"S-1-5-32-545": {
		SID:         "S-1-5-32-545",
		AccountName: "BUILTIN\\Users",
		Domain:      "BUILTIN",
		Username:    "Users",
		AccountType: "Alias",
	},
	"S-1-5-32-546": {
		SID:         "S-1-5-32-546",
		AccountName: "BUILTIN\\Guests",
		Domain:      "BUILTIN",
		Username:    "Guests",
		AccountType: "Alias",
	},
	"S-1-5-32-547": {
		SID:         "S-1-5-32-547",
		AccountName: "BUILTIN\\Power Users",
		Domain:      "BUILTIN",
		Username:    "Power Users",
		AccountType: "Alias",
	},
	"S-1-5-32-548": {
		SID:         "S-1-5-32-548",
		AccountName: "BUILTIN\\Account Operators",
		Domain:      "BUILTIN",
		Username:    "Account Operators",
		AccountType: "Alias",
	},
	"S-1-5-32-549": {
		SID:         "S-1-5-32-549",
		AccountName: "BUILTIN\\Server Operators",
		Domain:      "BUILTIN",
		Username:    "Server Operators",
		AccountType: "Alias",
	},
	"S-1-5-32-550": {
		SID:         "S-1-5-32-550",
		AccountName: "BUILTIN\\Print Operators",
		Domain:      "BUILTIN",
		Username:    "Print Operators",
		AccountType: "Alias",
	},
	"S-1-5-32-551": {
		SID:         "S-1-5-32-551",
		AccountName: "BUILTIN\\Backup Operators",
		Domain:      "BUILTIN",
		Username:    "Backup Operators",
		AccountType: "Alias",
	},
	"S-1-5-32-552": {
		SID:         "S-1-5-32-552",
		AccountName: "BUILTIN\\Replicators",
		Domain:      "BUILTIN",
		Username:    "Replicators",
		AccountType: "Alias",
	},
	"S-1-5-32-554": {
		SID:         "S-1-5-32-554",
		AccountName: "BUILTIN\\Pre-Windows 2000 Compatible Access",
		Domain:      "BUILTIN",
		Username:    "Pre-Windows 2000 Compatible Access",
		AccountType: "Alias",
	},
	"S-1-5-32-555": {
		SID:         "S-1-5-32-555",
		AccountName: "BUILTIN\\Remote Desktop Users",
		Domain:      "BUILTIN",
		Username:    "Remote Desktop Users",
		AccountType: "Alias",
	},
	"S-1-5-32-556": {
		SID:         "S-1-5-32-556",
		AccountName: "BUILTIN\\Network Configuration Operators",
		Domain:      "BUILTIN",
		Username:    "Network Configuration Operators",
		AccountType: "Alias",
	},
	"S-1-5-32-558": {
		SID:         "S-1-5-32-558",
		AccountName: "BUILTIN\\Performance Monitor Users",
		Domain:      "BUILTIN",
		Username:    "Performance Monitor Users",
		AccountType: "Alias",
	},
	"S-1-5-32-559": {
		SID:         "S-1-5-32-559",
		AccountName: "BUILTIN\\Performance Log Users",
		Domain:      "BUILTIN",
		Username:    "Performance Log Users",
		AccountType: "Alias",
	},
	"S-1-5-32-562": {
		SID:         "S-1-5-32-562",
		AccountName: "BUILTIN\\Distributed COM Users",
		Domain:      "BUILTIN",
		Username:    "Distributed COM Users",
		AccountType: "Alias",
	},
	"S-1-5-32-568": {
		SID:         "S-1-5-32-568",
		AccountName: "BUILTIN\\IIS_IUSRS",
		Domain:      "BUILTIN",
		Username:    "IIS_IUSRS",
		AccountType: "Alias",
	},
	"S-1-5-32-569": {
		SID:         "S-1-5-32-569",
		AccountName: "BUILTIN\\Cryptographic Operators",
		Domain:      "BUILTIN",
		Username:    "Cryptographic Operators",
		AccountType: "Alias",
	},
	"S-1-5-32-573": {
		SID:         "S-1-5-32-573",
		AccountName: "BUILTIN\\Event Log Readers",
		Domain:      "BUILTIN",
		Username:    "Event Log Readers",
		AccountType: "Alias",
	},
	"S-1-5-32-574": {
		SID:         "S-1-5-32-574",
		AccountName: "BUILTIN\\Certificate Service DCOM Access",
		Domain:      "BUILTIN",
		Username:    "Certificate Service DCOM Access",
		AccountType: "Alias",
	},
	"S-1-5-32-575": {
		SID:         "S-1-5-32-575",
		AccountName: "BUILTIN\\RDS Remote Access Servers",
		Domain:      "BUILTIN",
		Username:    "RDS Remote Access Servers",
		AccountType: "Alias",
	},
	"S-1-5-32-576": {
		SID:         "S-1-5-32-576",
		AccountName: "BUILTIN\\RDS Endpoint Servers",
		Domain:      "BUILTIN",
		Username:    "RDS Endpoint Servers",
		AccountType: "Alias",
	},
	"S-1-5-32-577": {
		SID:         "S-1-5-32-577",
		AccountName: "BUILTIN\\RDS Management Servers",
		Domain:      "BUILTIN",
		Username:    "RDS Management Servers",
		AccountType: "Alias",
	},
	"S-1-5-32-578": {
		SID:         "S-1-5-32-578",
		AccountName: "BUILTIN\\Hyper-V Administrators",
		Domain:      "BUILTIN",
		Username:    "Hyper-V Administrators",
		AccountType: "Alias",
	},
	"S-1-5-32-579": {
		SID:         "S-1-5-32-579",
		AccountName: "BUILTIN\\Access Control Assistance Operators",
		Domain:      "BUILTIN",
		Username:    "Access Control Assistance Operators",
		AccountType: "Alias",
	},
	"S-1-5-32-580": {
		SID:         "S-1-5-32-580",
		AccountName: "BUILTIN\\Remote Management Users",
		Domain:      "BUILTIN",
		Username:    "Remote Management Users",
		AccountType: "Alias",
	},

	// Other NT AUTHORITY accounts
	"S-1-5-7": {
		SID:         "S-1-5-7",
		AccountName: "NT AUTHORITY\\ANONYMOUS LOGON",
		Domain:      "NT AUTHORITY",
		Username:    "ANONYMOUS LOGON",
		AccountType: "WellKnownGroup",
	},
	"S-1-5-11": {
		SID:         "S-1-5-11",
		AccountName: "NT AUTHORITY\\Authenticated Users",
		Domain:      "NT AUTHORITY",
		Username:    "Authenticated Users",
		AccountType: "WellKnownGroup",
	},
	"S-1-5-32": {
		SID:         "S-1-5-32",
		AccountName: "BUILTIN",
		Domain:      "BUILTIN",
		Username:    "BUILTIN",
		AccountType: "WellKnownGroup",
	},
}

// isWellKnownSID checks if a SID is well-known and returns its resolved information
func isWellKnownSID(sid string) (*ResolvedSID, bool) {
	resolved, ok := wellKnownSIDs[sid]
	return resolved, ok
}
