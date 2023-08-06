// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package activedirectorydsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/activedirectorydsreceiver"

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/activedirectorydsreceiver/internal/metadata"
)

type activeDirectoryDSScraper struct {
	mb *metadata.MetricsBuilder
	w  *watchers
}

func newActiveDirectoryDSScraper(mbc metadata.MetricsBuilderConfig, params receiver.CreateSettings) *activeDirectoryDSScraper {
	return &activeDirectoryDSScraper{
		mb: metadata.NewMetricsBuilder(mbc, params),
	}
}

func (a *activeDirectoryDSScraper) start(ctx context.Context, host component.Host) error {
	watchers, err := getWatchers(defaultWatcherCreater{})
	if err != nil {
		return fmt.Errorf("failed to create performance counter watchers: %w", err)
	}
	a.w = watchers
	return nil
}

func (a *activeDirectoryDSScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	var multiErr error
	now := pcommon.NewTimestampFromTime(time.Now())
	rmb := a.mb.ResourceMetricsBuilder(pcommon.NewResource())

	draInboundBytesCompressed, err := a.w.Scrape(draInboundBytesCompressed)
	multiErr = multierr.Append(multiErr, err)
	if err == nil {
		rmb.RecordActiveDirectoryDsReplicationNetworkIoDataPoint(now, int64(draInboundBytesCompressed),
			metadata.AttributeDirectionReceived, metadata.AttributeNetworkDataTypeCompressed)
	}

	draInboundBytesNotCompressed, err := a.w.Scrape(draInboundBytesNotCompressed)
	multiErr = multierr.Append(multiErr, err)
	if err == nil {
		rmb.RecordActiveDirectoryDsReplicationNetworkIoDataPoint(now, int64(draInboundBytesNotCompressed),
			metadata.AttributeDirectionReceived, metadata.AttributeNetworkDataTypeUncompressed)
	}

	draOutboundBytesCompressed, err := a.w.Scrape(draOutboundBytesCompressed)
	multiErr = multierr.Append(multiErr, err)
	if err == nil {
		rmb.RecordActiveDirectoryDsReplicationNetworkIoDataPoint(now, int64(draOutboundBytesCompressed),
			metadata.AttributeDirectionSent, metadata.AttributeNetworkDataTypeCompressed)
	}

	draOutboundBytesNotCompressed, err := a.w.Scrape(draOutboundBytesNotCompressed)
	multiErr = multierr.Append(multiErr, err)
	if err == nil {
		rmb.RecordActiveDirectoryDsReplicationNetworkIoDataPoint(now, int64(draOutboundBytesNotCompressed),
			metadata.AttributeDirectionSent, metadata.AttributeNetworkDataTypeUncompressed)
	}

	draInboundFullSyncObjectsRemaining, err := a.w.Scrape(draInboundFullSyncObjectsRemaining)
	multiErr = multierr.Append(multiErr, err)
	if err == nil {
		rmb.RecordActiveDirectoryDsReplicationSyncObjectPendingDataPoint(now, int64(draInboundFullSyncObjectsRemaining))
	}

	draInboundObjects, err := a.w.Scrape(draInboundObjects)
	multiErr = multierr.Append(multiErr, err)
	if err == nil {
		rmb.RecordActiveDirectoryDsReplicationObjectRateDataPoint(now, draInboundObjects,
			metadata.AttributeDirectionReceived)
	}

	draOutboundObjects, err := a.w.Scrape(draOutboundObjects)
	multiErr = multierr.Append(multiErr, err)
	if err == nil {
		rmb.RecordActiveDirectoryDsReplicationObjectRateDataPoint(now, draOutboundObjects,
			metadata.AttributeDirectionSent)
	}

	draInboundProperties, err := a.w.Scrape(draInboundProperties)
	multiErr = multierr.Append(multiErr, err)
	if err == nil {
		rmb.RecordActiveDirectoryDsReplicationPropertyRateDataPoint(now, draInboundProperties,
			metadata.AttributeDirectionReceived)
	}

	draOutboundProperties, err := a.w.Scrape(draOutboundProperties)
	multiErr = multierr.Append(multiErr, err)
	if err == nil {
		rmb.RecordActiveDirectoryDsReplicationPropertyRateDataPoint(now, draOutboundProperties,
			metadata.AttributeDirectionSent)
	}

	draInboundValuesDNs, dnsErr := a.w.Scrape(draInboundValuesDNs)
	multiErr = multierr.Append(multiErr, dnsErr)
	if dnsErr == nil {
		rmb.RecordActiveDirectoryDsReplicationValueRateDataPoint(now, draInboundValuesDNs,
			metadata.AttributeDirectionReceived, metadata.AttributeValueTypeDistingushedNames)
	}

	draInboundValuesTotal, totalErr := a.w.Scrape(draInboundValuesTotal)
	multiErr = multierr.Append(multiErr, totalErr)
	if dnsErr == nil && totalErr == nil {
		otherValuesInbound := draInboundValuesTotal - draInboundValuesDNs
		rmb.RecordActiveDirectoryDsReplicationValueRateDataPoint(now, otherValuesInbound,
			metadata.AttributeDirectionReceived, metadata.AttributeValueTypeOther)
	}

	draOutboundValuesDNs, dnsErr := a.w.Scrape(draOutboundValuesDNs)
	multiErr = multierr.Append(multiErr, dnsErr)
	if dnsErr == nil {
		rmb.RecordActiveDirectoryDsReplicationValueRateDataPoint(now, draOutboundValuesDNs,
			metadata.AttributeDirectionSent, metadata.AttributeValueTypeDistingushedNames)
	}

	draOutboundValuesTotal, totalErr := a.w.Scrape(draOutboundValuesTotal)
	multiErr = multierr.Append(multiErr, totalErr)
	if dnsErr == nil && totalErr == nil {
		otherValuesOutbound := draOutboundValuesTotal - draOutboundValuesDNs
		rmb.RecordActiveDirectoryDsReplicationValueRateDataPoint(now, otherValuesOutbound,
			metadata.AttributeDirectionSent, metadata.AttributeValueTypeOther)
	}

	draPendingReplicationOperations, err := a.w.Scrape(draPendingReplicationOperations)
	multiErr = multierr.Append(multiErr, err)
	if err == nil {
		rmb.RecordActiveDirectoryDsReplicationOperationPendingDataPoint(now, int64(draPendingReplicationOperations))
	}

	draSyncFailuresSchemaMistmatch, schemaMismatchErr := a.w.Scrape(draSyncFailuresSchemaMismatch)
	multiErr = multierr.Append(multiErr, schemaMismatchErr)
	if schemaMismatchErr == nil {
		rmb.RecordActiveDirectoryDsReplicationSyncRequestCountDataPoint(now, int64(draSyncFailuresSchemaMistmatch),
			metadata.AttributeSyncResultSchemaMismatch)
	}

	draSyncRequestsSuccessful, requestsSuccessfulErr := a.w.Scrape(draSyncRequestsSuccessful)
	multiErr = multierr.Append(multiErr, requestsSuccessfulErr)
	if requestsSuccessfulErr == nil {
		rmb.RecordActiveDirectoryDsReplicationSyncRequestCountDataPoint(now, int64(draSyncRequestsSuccessful),
			metadata.AttributeSyncResultSuccess)
	}

	draSyncRequestsTotal, totalErr := a.w.Scrape(draSyncRequestsMade)
	multiErr = multierr.Append(multiErr, totalErr)
	if totalErr == nil && requestsSuccessfulErr == nil && schemaMismatchErr == nil {
		otherReplicationSyncRequests := draSyncRequestsTotal - draSyncRequestsSuccessful - draSyncFailuresSchemaMistmatch
		rmb.RecordActiveDirectoryDsReplicationSyncRequestCountDataPoint(now, int64(otherReplicationSyncRequests),
			metadata.AttributeSyncResultOther)
	}

	dsDirectoryReads, err := a.w.Scrape(dsDirectoryReads)
	multiErr = multierr.Append(multiErr, err)
	if err == nil {
		rmb.RecordActiveDirectoryDsOperationRateDataPoint(now, dsDirectoryReads, metadata.AttributeOperationTypeRead)
	}

	dsDirectoryWrites, err := a.w.Scrape(dsDirectoryWrites)
	multiErr = multierr.Append(multiErr, err)
	if err == nil {
		rmb.RecordActiveDirectoryDsOperationRateDataPoint(now, dsDirectoryWrites, metadata.AttributeOperationTypeWrite)
	}

	dsDirectorySearches, err := a.w.Scrape(dsDirectorySearches)
	multiErr = multierr.Append(multiErr, err)
	if err == nil {
		rmb.RecordActiveDirectoryDsOperationRateDataPoint(now, dsDirectorySearches,
			metadata.AttributeOperationTypeSearch)
	}

	dsClientBinds, err := a.w.Scrape(dsClientBinds)
	multiErr = multierr.Append(multiErr, err)
	if err == nil {
		rmb.RecordActiveDirectoryDsBindRateDataPoint(now, dsClientBinds, metadata.AttributeBindTypeClient)
	}

	dsServerBinds, err := a.w.Scrape(dsServerBinds)
	multiErr = multierr.Append(multiErr, err)
	if err == nil {
		rmb.RecordActiveDirectoryDsBindRateDataPoint(now, dsServerBinds, metadata.AttributeBindTypeServer)
	}

	dsCacheHitRate, err := a.w.Scrape(dsNameCacheHitRate)
	multiErr = multierr.Append(multiErr, err)
	if err == nil {
		rmb.RecordActiveDirectoryDsNameCacheHitRateDataPoint(now, dsCacheHitRate)
	}

	dsNotifyQueueSize, err := a.w.Scrape(dsNotifyQueueSize)
	multiErr = multierr.Append(multiErr, err)
	if err == nil {
		rmb.RecordActiveDirectoryDsNotificationQueuedDataPoint(now, int64(dsNotifyQueueSize))
	}

	securityPropEvents, err := a.w.Scrape(dsSecurityDescriptorPropagationsEvents)
	multiErr = multierr.Append(multiErr, err)
	if err == nil {
		rmb.RecordActiveDirectoryDsSecurityDescriptorPropagationsEventQueuedDataPoint(now, int64(securityPropEvents))
	}

	securityDescSubops, err := a.w.Scrape(dsSecurityDescripterSubOperations)
	multiErr = multierr.Append(multiErr, err)
	if err == nil {
		rmb.RecordActiveDirectoryDsSuboperationRateDataPoint(now, securityDescSubops,
			metadata.AttributeSuboperationTypeSecurityDescriptorPropagationsEvent)
	}

	searchSubops, err := a.w.Scrape(dsSearchSubOperations)
	multiErr = multierr.Append(multiErr, err)
	if err == nil {
		rmb.RecordActiveDirectoryDsSuboperationRateDataPoint(now, searchSubops,
			metadata.AttributeSuboperationTypeSearch)
	}

	threadsInUse, err := a.w.Scrape(dsThreadsInUse)
	multiErr = multierr.Append(multiErr, err)
	if err == nil {
		rmb.RecordActiveDirectoryDsThreadCountDataPoint(now, int64(threadsInUse))
	}

	ldapClientSessions, err := a.w.Scrape(ldapClientSessions)
	multiErr = multierr.Append(multiErr, err)
	if err == nil {
		rmb.RecordActiveDirectoryDsLdapClientSessionCountDataPoint(now, int64(ldapClientSessions))
	}

	ldapBindTime, err := a.w.Scrape(ldapBindTime)
	multiErr = multierr.Append(multiErr, err)
	if err == nil {
		rmb.RecordActiveDirectoryDsLdapBindLastSuccessfulTimeDataPoint(now, int64(ldapBindTime))
	}

	ldapSuccessfulBinds, err := a.w.Scrape(ldapSuccessfulBinds)
	multiErr = multierr.Append(multiErr, err)
	if err == nil {
		rmb.RecordActiveDirectoryDsLdapBindRateDataPoint(now, ldapSuccessfulBinds)
	}

	ldapSearches, err := a.w.Scrape(ldapSearches)
	multiErr = multierr.Append(multiErr, err)
	if err == nil {
		rmb.RecordActiveDirectoryDsLdapSearchRateDataPoint(now, ldapSearches)
	}

	if multiErr != nil {
		return pmetric.Metrics(a.mb.Emit()), scrapererror.NewPartialScrapeError(multiErr, len(multierr.Errors(multiErr)))
	}

	return pmetric.Metrics(a.mb.Emit()), nil
}

func (a *activeDirectoryDSScraper) shutdown(_ context.Context) error {
	return a.w.Close()
}
