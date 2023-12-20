// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package activedirectorydsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/activedirectorydsreceiver"

import (
	"fmt"

	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters"
)

const (
	draInboundBytesCompressed              = "DRA Inbound Bytes Compressed (Between Sites, After Compression) Since Boot"
	draInboundBytesNotCompressed           = "DRA Inbound Bytes Not Compressed (Within Site) Since Boot"
	draOutboundBytesCompressed             = "DRA Outbound Bytes Compressed (Between Sites, After Compression) Since Boot"
	draOutboundBytesNotCompressed          = "DRA Outbound Bytes Not Compressed (Within Site) Since Boot"
	draInboundFullSyncObjectsRemaining     = "DRA Inbound Full Sync Objects Remaining"
	draInboundObjects                      = "DRA Inbound Objects/sec"
	draOutboundObjects                     = "DRA Outbound Objects/sec"
	draInboundProperties                   = "DRA Inbound Properties Total/sec"
	draOutboundProperties                  = "DRA Outbound Properties/sec"
	draInboundValuesDNs                    = "DRA Inbound Values (DNs only)/sec" //revive:disable-line:var-naming
	draInboundValuesTotal                  = "DRA Inbound Values Total/sec"
	draOutboundValuesDNs                   = "DRA Outbound Values (DNs only)/sec" //revive:disable-line:var-naming
	draOutboundValuesTotal                 = "DRA Outbound Values Total/sec"
	draPendingReplicationOperations        = "DRA Pending Replication Operations"
	draSyncFailuresSchemaMismatch          = "DRA Sync Failures on Schema Mismatch"
	draSyncRequestsSuccessful              = "DRA Sync Requests Successful"
	draSyncRequestsMade                    = "DRA Sync Requests Made"
	dsDirectoryReads                       = "DS Directory Reads/sec"
	dsDirectoryWrites                      = "DS Directory Writes/sec"
	dsDirectorySearches                    = "DS Directory Searches/sec"
	dsClientBinds                          = "DS Client Binds/sec"
	dsServerBinds                          = "DS Server Binds/sec"
	dsNameCacheHitRate                     = "DS Name Cache hit rate"
	dsNotifyQueueSize                      = "DS Notify Queue Size"
	dsSecurityDescriptorPropagationsEvents = "DS Security Descriptor Propagations Events"
	dsSearchSubOperations                  = "DS Search sub-operations/sec"
	dsSecurityDescripterSubOperations      = "DS Security Descriptor sub-operations/sec"
	dsThreadsInUse                         = "DS Threads in Use"
	ldapClientSessions                     = "LDAP Client Sessions"
	ldapBindTime                           = "LDAP Bind Time"
	ldapSuccessfulBinds                    = "LDAP Successful Binds/sec"
	ldapSearches                           = "LDAP Searches/sec"
)

type watchers struct {
	closed bool

	counterNameToWatcher map[string]winperfcounters.PerfCounterWatcher
}

func (w *watchers) Scrape(name string) (float64, error) {
	v, ok := w.counterNameToWatcher[name]
	if !ok {
		return 0, fmt.Errorf("counter \"%s\" was not initialized", name)
	}

	cv, err := v.ScrapeData()
	if err != nil {
		return 0, err
	}

	return cv[0].Value, nil
}

func (w *watchers) Close() error {
	if w == nil || w.closed {
		return nil
	}

	var err error
	for _, v := range w.counterNameToWatcher {
		err = multierr.Append(err, v.Close())
	}

	return err
}

func getWatchers(wc watcherCreater) (*watchers, error) {
	var err error

	w := &watchers{
		counterNameToWatcher: make(map[string]winperfcounters.PerfCounterWatcher),
	}

	if w.counterNameToWatcher[draInboundBytesCompressed], err = wc.Create(draInboundBytesCompressed); err != nil {
		return nil, err
	}

	if w.counterNameToWatcher[draInboundBytesNotCompressed], err = wc.Create(draInboundBytesNotCompressed); err != nil {
		return nil, err
	}

	if w.counterNameToWatcher[draOutboundBytesCompressed], err = wc.Create(draOutboundBytesCompressed); err != nil {
		return nil, err
	}

	if w.counterNameToWatcher[draOutboundBytesNotCompressed], err = wc.Create(draOutboundBytesNotCompressed); err != nil {
		return nil, err
	}

	if w.counterNameToWatcher[draInboundFullSyncObjectsRemaining], err = wc.Create(draInboundFullSyncObjectsRemaining); err != nil {
		return nil, err
	}

	if w.counterNameToWatcher[draInboundObjects], err = wc.Create(draInboundObjects); err != nil {
		return nil, err
	}

	if w.counterNameToWatcher[draOutboundObjects], err = wc.Create(draOutboundObjects); err != nil {
		return nil, err
	}

	if w.counterNameToWatcher[draInboundProperties], err = wc.Create(draInboundProperties); err != nil {
		return nil, err
	}

	if w.counterNameToWatcher[draOutboundProperties], err = wc.Create(draOutboundProperties); err != nil {
		return nil, err
	}

	if w.counterNameToWatcher[draInboundValuesDNs], err = wc.Create(draInboundValuesDNs); err != nil {
		return nil, err
	}

	if w.counterNameToWatcher[draInboundValuesTotal], err = wc.Create(draInboundValuesTotal); err != nil {
		return nil, err
	}

	if w.counterNameToWatcher[draOutboundValuesDNs], err = wc.Create(draOutboundValuesDNs); err != nil {
		return nil, err
	}

	if w.counterNameToWatcher[draOutboundValuesTotal], err = wc.Create(draOutboundValuesTotal); err != nil {
		return nil, err
	}

	if w.counterNameToWatcher[draPendingReplicationOperations], err = wc.Create(draPendingReplicationOperations); err != nil {
		return nil, err
	}

	if w.counterNameToWatcher[draSyncFailuresSchemaMismatch], err = wc.Create(draSyncFailuresSchemaMismatch); err != nil {
		return nil, err
	}

	if w.counterNameToWatcher[draSyncRequestsSuccessful], err = wc.Create(draSyncRequestsSuccessful); err != nil {
		return nil, err
	}

	if w.counterNameToWatcher[draSyncRequestsMade], err = wc.Create(draSyncRequestsMade); err != nil {
		return nil, err
	}

	if w.counterNameToWatcher[dsDirectoryReads], err = wc.Create(dsDirectoryReads); err != nil {
		return nil, err
	}

	if w.counterNameToWatcher[dsDirectoryWrites], err = wc.Create(dsDirectoryWrites); err != nil {
		return nil, err
	}

	if w.counterNameToWatcher[dsDirectorySearches], err = wc.Create(dsDirectorySearches); err != nil {
		return nil, err
	}

	if w.counterNameToWatcher[dsClientBinds], err = wc.Create(dsClientBinds); err != nil {
		return nil, err
	}

	if w.counterNameToWatcher[dsServerBinds], err = wc.Create(dsServerBinds); err != nil {
		return nil, err
	}

	if w.counterNameToWatcher[dsNameCacheHitRate], err = wc.Create(dsNameCacheHitRate); err != nil {
		return nil, err
	}

	if w.counterNameToWatcher[dsNotifyQueueSize], err = wc.Create(dsNotifyQueueSize); err != nil {
		return nil, err
	}

	if w.counterNameToWatcher[dsSecurityDescriptorPropagationsEvents], err = wc.Create(dsSecurityDescriptorPropagationsEvents); err != nil {
		return nil, err
	}

	if w.counterNameToWatcher[dsSearchSubOperations], err = wc.Create(dsSearchSubOperations); err != nil {
		return nil, err
	}

	if w.counterNameToWatcher[dsSecurityDescripterSubOperations], err = wc.Create(dsSecurityDescripterSubOperations); err != nil {
		return nil, err
	}

	if w.counterNameToWatcher[dsThreadsInUse], err = wc.Create(dsThreadsInUse); err != nil {
		return nil, err
	}

	if w.counterNameToWatcher[ldapClientSessions], err = wc.Create(ldapClientSessions); err != nil {
		return nil, err
	}

	if w.counterNameToWatcher[ldapBindTime], err = wc.Create(ldapBindTime); err != nil {
		return nil, err
	}

	if w.counterNameToWatcher[ldapSuccessfulBinds], err = wc.Create(ldapSuccessfulBinds); err != nil {
		return nil, err
	}

	if w.counterNameToWatcher[ldapSearches], err = wc.Create(ldapSearches); err != nil {
		return nil, err
	}

	return w, nil
}

type watcherCreater interface {
	Create(counterName string) (winperfcounters.PerfCounterWatcher, error)
}

const (
	instanceName = "NTDS"
	object       = "DirectoryServices"
)

type defaultWatcherCreater struct{}

func (defaultWatcherCreater) Create(counterName string) (winperfcounters.PerfCounterWatcher, error) {
	return winperfcounters.NewWatcher(object, instanceName, counterName)
}
