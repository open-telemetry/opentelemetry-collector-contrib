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

func getWatchers(wc watcherCreater) (*watchers, error) {
	DRAInboundBytesCompressed, err := wc.Create("DRA Inbound Bytes Compressed (Between Sites, After Compression) Since Boot")
	if err != nil {
		return nil, err
	}

	DRAInboundBytesNotCompressed, err := wc.Create("DRA Inbound Bytes Not Compressed (Within Site) Since Boot")
	if err != nil {
		return nil, err
	}

	DRAOutboundBytesCompressed, err := wc.Create("DRA Outbound Bytes Compressed (Between Sites, After Compression) Since Boot")
	if err != nil {
		return nil, err
	}

	DRAOutboundBytesNotCompressed, err := wc.Create("DRA Outbound Bytes Not Compressed (Within Site) Since Boot")
	if err != nil {
		return nil, err
	}

	DRAInboundFullSyncObjectsRemaining, err := wc.Create("DRA Inbound Full Sync Objects Remaining")
	if err != nil {
		return nil, err
	}

	DRAInboundObjects, err := wc.Create("DRA Inbound Objects/sec")
	if err != nil {
		return nil, err
	}

	DRAOutboundObjects, err := wc.Create("DRA Outbound Objects/sec")
	if err != nil {
		return nil, err
	}

	DRAInboundProperties, err := wc.Create("DRA Inbound Properties Total/sec")
	if err != nil {
		return nil, err
	}

	DRAOutboundProperties, err := wc.Create("DRA Outbound Properties/sec")
	if err != nil {
		return nil, err
	}

	DRAInboundValuesDNs, err := wc.Create("DRA Inbound Values (DNs only)/sec")
	if err != nil {
		return nil, err
	}

	DRAInboundValuesTotal, err := wc.Create("DRA Inbound Values Total/sec")
	if err != nil {
		return nil, err
	}

	DRAOutboundValuesDNs, err := wc.Create("DRA Outbound Values (DNs only)/sec")
	if err != nil {
		return nil, err
	}

	DRAOutboundValuesTotal, err := wc.Create("DRA Outbound Values Total/sec")
	if err != nil {
		return nil, err
	}

	DRAPendingReplicationOperations, err := wc.Create("DRA Pending Replication Operations")
	if err != nil {
		return nil, err
	}

	DRASyncFailuresSchemaMismatch, err := wc.Create("DRA Sync Failures on Schema Mismatch")
	if err != nil {
		return nil, err
	}

	DRASyncRequestsSuccessful, err := wc.Create("DRA Sync Requests Successful")
	if err != nil {
		return nil, err
	}

	DRASyncRequestsMade, err := wc.Create("DRA Sync Requests Made")
	if err != nil {
		return nil, err
	}

	DSDirectoryReads, err := wc.Create("DS Directory Reads/sec")
	if err != nil {
		return nil, err
	}

	DSDirectoryWrites, err := wc.Create("DS Directory Writes/sec")
	if err != nil {
		return nil, err
	}

	DSDirectorySearches, err := wc.Create("DS Directory Searches/sec")
	if err != nil {
		return nil, err
	}

	DSClientBinds, err := wc.Create("DS Client Binds/sec")
	if err != nil {
		return nil, err
	}

	DSServerBinds, err := wc.Create("DS Server Binds/sec")
	if err != nil {
		return nil, err
	}

	DSNameCacheHitRate, err := wc.Create("DS Name Cache hit rate")
	if err != nil {
		return nil, err
	}

	DSNotifyQueueSize, err := wc.Create("DS Notify Queue Size")
	if err != nil {
		return nil, err
	}

	DSSecurityDescriptorPropagationsEvents, err := wc.Create("DS Security Descriptor Propagations Events")
	if err != nil {
		return nil, err
	}

	DSSearchSubOperations, err := wc.Create("DS Search sub-operations/sec")
	if err != nil {
		return nil, err
	}

	DSSecurityDescripterSubOperations, err := wc.Create("DS Security Descriptor sub-operations/sec")
	if err != nil {
		return nil, err
	}

	DSThreadsInUse, err := wc.Create("DS Threads in Use")
	if err != nil {
		return nil, err
	}

	LDAPClientSessions, err := wc.Create("LDAP Client Sessions")
	if err != nil {
		return nil, err
	}

	LDAPBindTime, err := wc.Create("LDAP Bind Time")
	if err != nil {
		return nil, err
	}

	LDAPSuccessfulBinds, err := wc.Create("LDAP Successful Binds/sec")
	if err != nil {
		return nil, err
	}

	LDAPSearches, err := wc.Create("LDAP Searches/sec")
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

type watcherCreater interface {
	Create(counterName string) (winperfcounters.PerfCounterWatcher, error)
}

const (
	instanceName = "NTDS"
	object       = "DirectoryServices"
)

type defaultWatcherCreater struct {}

func (defaultWatcherCreater) Create(counterName string) (winperfcounters.PerfCounterWatcher, error) {
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
