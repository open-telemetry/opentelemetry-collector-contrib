// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build windows
// +build windows

package activedirectorydsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/activedirectorydsreceiver"

import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters"

type watchers struct {
	DRAInboundBytesCompressed              winperfcounters.PerfCounterWatcher
	DRAInboundBytesNotCompressed           winperfcounters.PerfCounterWatcher
	DRAOutboundBytesCompressed             winperfcounters.PerfCounterWatcher
	DRAOutboundBytesNotCompressed          winperfcounters.PerfCounterWatcher
	DRAInboundFullSyncObjectsRemaining     winperfcounters.PerfCounterWatcher
	DRAInboundObjects                      winperfcounters.PerfCounterWatcher
	DRAOutboundObjects                     winperfcounters.PerfCounterWatcher
	DRAInboundProperties                   winperfcounters.PerfCounterWatcher
	DRAOutboundProperties                  winperfcounters.PerfCounterWatcher
	DRAInboundValuesDNs                    winperfcounters.PerfCounterWatcher
	DRAInboundValuesTotal                  winperfcounters.PerfCounterWatcher
	DRAOutboundValuesDNs                   winperfcounters.PerfCounterWatcher
	DRAOutboundValuesTotal                 winperfcounters.PerfCounterWatcher
	DRAPendingReplicationOperations        winperfcounters.PerfCounterWatcher
	DRASyncFailuresSchemaMismatch          winperfcounters.PerfCounterWatcher
	DRASyncRequestsSuccessful              winperfcounters.PerfCounterWatcher
	DRASyncRequestsMade                    winperfcounters.PerfCounterWatcher
	DSDirectoryReads                       winperfcounters.PerfCounterWatcher
	DSDirectoryWrites                      winperfcounters.PerfCounterWatcher
	DSDirectorySearches                    winperfcounters.PerfCounterWatcher
	DSClientBinds                          winperfcounters.PerfCounterWatcher
	DSServerBinds                          winperfcounters.PerfCounterWatcher
	DSNameCacheHitRate                     winperfcounters.PerfCounterWatcher
	DSNotifyQueueSize                      winperfcounters.PerfCounterWatcher
	DSSecurityDescriptorPropagationsEvents winperfcounters.PerfCounterWatcher
	DSSearchSubOperations                  winperfcounters.PerfCounterWatcher
	DSSecurityDescripterSubOperations      winperfcounters.PerfCounterWatcher
	DSThreadsInUse                         winperfcounters.PerfCounterWatcher
	LDAPClientSessions                     winperfcounters.PerfCounterWatcher
	LDAPBindTime                           winperfcounters.PerfCounterWatcher
	LDAPSuccessfulBinds                    winperfcounters.PerfCounterWatcher
	LDAPSearches                           winperfcounters.PerfCounterWatcher
}

func getWatchers() (*watchers, error) {
	DRAInboundBytesCompressed, err := createWatcher("DRA Inbound Bytes Compressed (Between Sites, After Compression) Since Boot")
	if err != nil {
		return nil, err
	}

	DRAInboundBytesNotCompressed, err := createWatcher("DRA Inbound Bytes Not Compressed (Within Site) Since Boot")
	if err != nil {
		return nil, err
	}

	DRAOutboundBytesCompressed, err := createWatcher("DRA Outbound Bytes Compressed (Between Sites, After Compression) Since Boot")
	if err != nil {
		return nil, err
	}

	DRAOutboundBytesNotCompressed, err := createWatcher("DRA Outbound Bytes Not Compressed (Within Site) Since Boot")
	if err != nil {
		return nil, err
	}

	DRAInboundFullSyncObjectsRemaining, err := createWatcher("DRA Inbound Full Sync Objects Remaining")
	if err != nil {
		return nil, err
	}

	DRAInboundObjects, err := createWatcher("DRA Inbound Objects/sec")
	if err != nil {
		return nil, err
	}

	DRAOutboundObjects, err := createWatcher("DRA Outbound Objects/sec")
	if err != nil {
		return nil, err
	}

	DRAInboundProperties, err := createWatcher("DRA Inbound Properties Total/sec")
	if err != nil {
		return nil, err
	}

	DRAOutboundProperties, err := createWatcher("DRA Outbound Properties/sec")
	if err != nil {
		return nil, err
	}

	DRAInboundValuesDNs, err := createWatcher("DRA Inbound Values (DNs only)/sec")
	if err != nil {
		return nil, err
	}

	DRAInboundValuesTotal, err := createWatcher("DRA Inbound Values Total/sec")
	if err != nil {
		return nil, err
	}

	DRAOutboundValuesDNs, err := createWatcher("DRA Outbound Values (DNs only)/sec")
	if err != nil {
		return nil, err
	}

	DRAOutboundValuesTotal, err := createWatcher("DRA Outbound Values Total/sec")
	if err != nil {
		return nil, err
	}

	DRAPendingReplicationOperations, err := createWatcher("DRA Pending Replication Operations")
	if err != nil {
		return nil, err
	}

	DRASyncFailuresSchemaMismatch, err := createWatcher("DRA Sync Failures on Schema Mismatch")
	if err != nil {
		return nil, err
	}

	DRASyncRequestsSuccessful, err := createWatcher("DRA Sync Requests Successful")
	if err != nil {
		return nil, err
	}

	DRASyncRequestsMade, err := createWatcher("DRA Sync Requests Made")
	if err != nil {
		return nil, err
	}

	DSDirectoryReads, err := createWatcher("DS Directory Reads/sec")
	if err != nil {
		return nil, err
	}

	DSDirectoryWrites, err := createWatcher("DS Directory Writes/sec")
	if err != nil {
		return nil, err
	}

	DSDirectorySearches, err := createWatcher("DS Directory Searches/sec")
	if err != nil {
		return nil, err
	}

	DSClientBinds, err := createWatcher("DS Client Binds/sec")
	if err != nil {
		return nil, err
	}

	DSServerBinds, err := createWatcher("DS Server Binds/sec")
	if err != nil {
		return nil, err
	}

	DSNameCacheHitRate, err := createWatcher("DS Name Cache hit rate")
	if err != nil {
		return nil, err
	}

	DSNotifyQueueSize, err := createWatcher("DS Notify Queue Size")
	if err != nil {
		return nil, err
	}

	DSSecurityDescriptorPropagationsEvents, err := createWatcher("DS Security Descriptor Propagations Events")
	if err != nil {
		return nil, err
	}

	DSSearchSubOperations, err := createWatcher("DS Search sub-operations/sec")
	if err != nil {
		return nil, err
	}

	DSSecurityDescripterSubOperations, err := createWatcher("DS Security Descriptor sub-operations/sec")
	if err != nil {
		return nil, err
	}

	DSThreadsInUse, err := createWatcher("DS Threads in Use")
	if err != nil {
		return nil, err
	}

	LDAPClientSessions, err := createWatcher("LDAP Client Sessions")
	if err != nil {
		return nil, err
	}

	LDAPBindTime, err := createWatcher("LDAP Bind Time")
	if err != nil {
		return nil, err
	}

	LDAPSuccessfulBinds, err := createWatcher("LDAP Successful Binds/sec")
	if err != nil {
		return nil, err
	}

	LDAPSearches, err := createWatcher("LDAP Searches/sec")
	if err != nil {
		return nil, err
	}

	return &watchers{
		DRAInboundBytesCompressed:              DRAInboundBytesCompressed,
		DRAInboundBytesNotCompressed:           DRAInboundBytesNotCompressed,
		DRAOutboundBytesCompressed:             DRAOutboundBytesCompressed,
		DRAOutboundBytesNotCompressed:          DRAOutboundBytesNotCompressed,
		DRAInboundFullSyncObjectsRemaining:     DRAInboundFullSyncObjectsRemaining,
		DRAInboundObjects:                      DRAInboundObjects,
		DRAOutboundObjects:                     DRAOutboundObjects,
		DRAInboundProperties:                   DRAInboundProperties,
		DRAOutboundProperties:                  DRAOutboundProperties,
		DRAInboundValuesDNs:                    DRAInboundValuesDNs,
		DRAInboundValuesTotal:                  DRAInboundValuesTotal,
		DRAOutboundValuesDNs:                   DRAOutboundValuesDNs,
		DRAOutboundValuesTotal:                 DRAOutboundValuesTotal,
		DRAPendingReplicationOperations:        DRAPendingReplicationOperations,
		DRASyncFailuresSchemaMismatch:          DRASyncFailuresSchemaMismatch,
		DRASyncRequestsSuccessful:              DRASyncRequestsSuccessful,
		DRASyncRequestsMade:                    DRASyncRequestsMade,
		DSDirectoryReads:                       DSDirectoryReads,
		DSDirectoryWrites:                      DSDirectoryWrites,
		DSDirectorySearches:                    DSDirectorySearches,
		DSClientBinds:                          DSClientBinds,
		DSServerBinds:                          DSServerBinds,
		DSNameCacheHitRate:                     DSNameCacheHitRate,
		DSNotifyQueueSize:                      DSNotifyQueueSize,
		DSSecurityDescriptorPropagationsEvents: DSSecurityDescriptorPropagationsEvents,
		DSSearchSubOperations:                  DSSearchSubOperations,
		DSSecurityDescripterSubOperations:      DSSecurityDescripterSubOperations,
		DSThreadsInUse:                         DSThreadsInUse,
		LDAPClientSessions:                     LDAPClientSessions,
		LDAPBindTime:                           LDAPBindTime,
		LDAPSuccessfulBinds:                    LDAPSuccessfulBinds,
		LDAPSearches:                           LDAPSearches,
	}, nil
}

const (
	instanceName = "NTDS"
	object       = "DirectoryServices"
)

func createWatcher(counterName string) (winperfcounters.PerfCounterWatcher, error) {
	conf := winperfcounters.ObjectConfig{
		Object:    object,
		Instances: []string{instanceName},
		Counters: []winperfcounters.CounterConfig{
			{
				Name: counterName,
			},
		},
	}

	watchers, err := conf.BuildPaths()
	if err != nil {
		return nil, err
	}

	return watchers[0], nil
}
