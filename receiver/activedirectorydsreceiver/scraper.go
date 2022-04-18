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

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/activedirectorydsreceiver/internal/metadata"
)

type activeDirectoryDSScraper struct {
	mb *metadata.MetricsBuilder
	w  *watchers
}

func newActiveDirectoryDSScraper(ms metadata.MetricsSettings) *activeDirectoryDSScraper {
	return &activeDirectoryDSScraper{
		mb: metadata.NewMetricsBuilder(ms),
	}
}

func (a *activeDirectoryDSScraper) start(ctx context.Context, host component.Host) error {
	watchers, err := getWatchers()
	if err != nil {
		return fmt.Errorf("failed to create performance counter watchers: %w", err)
	}

	a.w = watchers

	a.mb.Reset()

	return nil
}

func (a *activeDirectoryDSScraper) scrape(ctx context.Context) (pdata.Metrics, error) {
	var multiErr error
	now := pcommon.NewTimestampFromTime(time.Now())

	draInboundBytesCompressed, err := a.w.DRAInboundBytesCompressed.ScrapeData()
	multiErr = multierr.Append(multiErr, err)
	if err == nil {
		a.mb.RecordActiveDirectoryDsReplicationNetworkIoDataPoint(now, int64(draInboundBytesCompressed[0].Value), metadata.AttributeDirection.Received, metadata.AttributeNetworkDataType.Compressed)
	}

	draInboundBytesNotCompressed, err := a.w.DRAInboundBytesNotCompressed.ScrapeData()
	multiErr = multierr.Append(multiErr, err)
	if err == nil {
		a.mb.RecordActiveDirectoryDsReplicationNetworkIoDataPoint(now, int64(draInboundBytesNotCompressed[0].Value), metadata.AttributeDirection.Received, metadata.AttributeNetworkDataType.Uncompressed)
	}

	draOutboundBytesCompressed, err := a.w.DRAOutboundBytesCompressed.ScrapeData()
	multiErr = multierr.Append(multiErr, err)
	if err == nil {
		a.mb.RecordActiveDirectoryDsReplicationNetworkIoDataPoint(now, int64(draOutboundBytesCompressed[0].Value), metadata.AttributeDirection.Sent, metadata.AttributeNetworkDataType.Compressed)
	}

	draOutboundBytesNotCompressed, err := a.w.DRAOutboundBytesNotCompressed.ScrapeData()
	multiErr = multierr.Append(multiErr, err)
	if err == nil {
		a.mb.RecordActiveDirectoryDsReplicationNetworkIoDataPoint(now, int64(draOutboundBytesNotCompressed[0].Value), metadata.AttributeDirection.Sent, metadata.AttributeNetworkDataType.Uncompressed)
	}

	draInboundFullSyncObjectsRemaining, err := a.w.DRAInboundFullSyncObjectsRemaining.ScrapeData()
	multiErr = multierr.Append(multiErr, err)
	if err == nil {
		a.mb.RecordActiveDirectoryDsReplicationSyncObjectPendingDataPoint(now, int64(draInboundFullSyncObjectsRemaining[0].Value))
	}

	draInboundObjects, err := a.w.DRAInboundObjects.ScrapeData()
	multiErr = multierr.Append(multiErr, err)
	if err == nil {
		a.mb.RecordActiveDirectoryDsReplicationObjectRateDataPoint(now, draInboundObjects[0].Value, metadata.AttributeDirection.Received)
	}

	draOutboundObjects, err := a.w.DRAOutboundObjects.ScrapeData()
	multiErr = multierr.Append(multiErr, err)
	if err == nil {
		a.mb.RecordActiveDirectoryDsReplicationObjectRateDataPoint(now, draOutboundObjects[0].Value, metadata.AttributeDirection.Sent)
	}

	draInboundProperties, err := a.w.DRAInboundProperties.ScrapeData()
	multiErr = multierr.Append(multiErr, err)
	if err == nil {
		a.mb.RecordActiveDirectoryDsReplicationPropertyRateDataPoint(now, draInboundProperties[0].Value, metadata.AttributeDirection.Received)
	}

	draOutboundProperties, err := a.w.DRAOutboundProperties.ScrapeData()
	multiErr = multierr.Append(multiErr, err)
	if err == nil {
		a.mb.RecordActiveDirectoryDsReplicationPropertyRateDataPoint(now, draOutboundProperties[0].Value, metadata.AttributeDirection.Sent)
	}

	draInboundValuesDNs, dnsErr := a.w.DRAInboundValuesDNs.ScrapeData()
	multiErr = multierr.Append(multiErr, dnsErr)
	if dnsErr == nil {
		a.mb.RecordActiveDirectoryDsReplicationValueRateDataPoint(now, draInboundValuesDNs[0].Value, metadata.AttributeDirection.Received, metadata.AttributeValueType.DistingushedNames)
	}

	draInboundValuesTotal, totalErr := a.w.DRAInboundValuesTotal.ScrapeData()
	multiErr = multierr.Append(multiErr, totalErr)
	if dnsErr == nil && totalErr == nil {
		otherValuesInbound := draInboundValuesTotal[0].Value - draInboundValuesDNs[0].Value
		a.mb.RecordActiveDirectoryDsReplicationValueRateDataPoint(now, otherValuesInbound, metadata.AttributeDirection.Received, metadata.AttributeValueType.Other)
	}

	draOutboundValuesDNs, dnsErr := a.w.DRAOutboundValuesDNs.ScrapeData()
	multiErr = multierr.Append(multiErr, dnsErr)
	if dnsErr == nil {
		a.mb.RecordActiveDirectoryDsReplicationValueRateDataPoint(now, draOutboundValuesDNs[0].Value, metadata.AttributeDirection.Sent, metadata.AttributeValueType.DistingushedNames)
	}

	draOutboundValuesTotal, totalErr := a.w.DRAOutboundValuesTotal.ScrapeData()
	multiErr = multierr.Append(multiErr, totalErr)
	if dnsErr == nil && totalErr == nil {
		otherValuesOutbound := draOutboundValuesTotal[0].Value - draOutboundValuesDNs[0].Value
		a.mb.RecordActiveDirectoryDsReplicationValueRateDataPoint(now, otherValuesOutbound, metadata.AttributeDirection.Sent, metadata.AttributeValueType.Other)
	}

	draPendingReplicationOperations, err := a.w.DRAPendingReplicationOperations.ScrapeData()
	multiErr = multierr.Append(multiErr, err)
	if err == nil {
		a.mb.RecordActiveDirectoryDsReplicationOperationPendingDataPoint(now, int64(draPendingReplicationOperations[0].Value))
	}

	draSyncFailuresSchemaMistmatch, schemaMismatchErr := a.w.DRASyncFailuresSchemaMismatch.ScrapeData()
	multiErr = multierr.Append(multiErr, schemaMismatchErr)
	if schemaMismatchErr == nil {
		a.mb.RecordActiveDirectoryDsReplicationSyncRequestCountDataPoint(now, int64(draSyncFailuresSchemaMistmatch[0].Value), metadata.AttributeSyncResult.SchemaMismatch)
	}

	draSyncRequestsSuccessful, requestsSuccessfulErr := a.w.DRASyncRequestsSuccessful.ScrapeData()
	multiErr = multierr.Append(multiErr, requestsSuccessfulErr)
	if requestsSuccessfulErr == nil {
		a.mb.RecordActiveDirectoryDsReplicationSyncRequestCountDataPoint(now, int64(draSyncRequestsSuccessful[0].Value), metadata.AttributeSyncResult.Success)
	}

	draSyncRequestsTotal, totalErr := a.w.DRASyncRequestsMade.ScrapeData()
	multiErr = multierr.Append(multiErr, totalErr)
	if totalErr == nil && requestsSuccessfulErr == nil && schemaMismatchErr == nil {
		otherReplicationSyncRequests := draSyncRequestsTotal[0].Value - draSyncRequestsSuccessful[0].Value - draSyncFailuresSchemaMistmatch[0].Value
		a.mb.RecordActiveDirectoryDsReplicationSyncRequestCountDataPoint(now, int64(otherReplicationSyncRequests), metadata.AttributeSyncResult.Other)
	}

	dsDirectoryReads, err := a.w.DSDirectoryReads.ScrapeData()
	multiErr = multierr.Append(multiErr, err)
	if err == nil {
		a.mb.RecordActiveDirectoryDsOperationRateDataPoint(now, dsDirectoryReads[0].Value, metadata.AttributeOperationType.Read)
	}

	dsDirectoryWrites, err := a.w.DSDirectoryWrites.ScrapeData()
	multiErr = multierr.Append(multiErr, err)
	if err == nil {
		a.mb.RecordActiveDirectoryDsOperationRateDataPoint(now, dsDirectoryWrites[0].Value, metadata.AttributeOperationType.Write)
	}

	dsDirectorySearches, err := a.w.DSDirectorySearches.ScrapeData()
	multiErr = multierr.Append(multiErr, err)
	if err == nil {
		a.mb.RecordActiveDirectoryDsOperationRateDataPoint(now, dsDirectorySearches[0].Value, metadata.AttributeOperationType.Search)
	}

	dsClientBinds, err := a.w.DSClientBinds.ScrapeData()
	multiErr = multierr.Append(multiErr, err)
	if err == nil {
		a.mb.RecordActiveDirectoryDsBindRateDataPoint(now, dsClientBinds[0].Value, metadata.AttributeBindType.Client)
	}

	dsServerBinds, err := a.w.DSServerBinds.ScrapeData()
	multiErr = multierr.Append(multiErr, err)
	if err == nil {
		a.mb.RecordActiveDirectoryDsBindRateDataPoint(now, dsServerBinds[0].Value, metadata.AttributeBindType.Server)
	}

	dsCacheHitRate, err := a.w.DSNameCacheHitRate.ScrapeData()
	multiErr = multierr.Append(multiErr, err)
	if err == nil {
		a.mb.RecordActiveDirectoryDsNameCacheHitRateDataPoint(now, dsCacheHitRate[0].Value)
	}

	dsNotifyQueueSize, err := a.w.DSNotifyQueueSize.ScrapeData()
	multiErr = multierr.Append(multiErr, err)
	if err == nil {
		a.mb.RecordActiveDirectoryDsNotificationQueuedDataPoint(now, int64(dsNotifyQueueSize[0].Value))
	}

	securityPropEvents, err := a.w.DSSecurityDescriptorPropagationsEvents.ScrapeData()
	multiErr = multierr.Append(multiErr, err)
	if err == nil {
		a.mb.RecordActiveDirectoryDsSecurityDescriptorPropagationsEventQueuedDataPoint(now, int64(securityPropEvents[0].Value))
	}

	securityDescSubops, err := a.w.DSSecurityDescripterSubOperations.ScrapeData()
	multiErr = multierr.Append(multiErr, err)
	if err == nil {
		a.mb.RecordActiveDirectoryDsSuboperationRateDataPoint(now, securityDescSubops[0].Value, metadata.AttributeSuboperationType.SecurityDescriptorPropagationsEvent)
	}

	searchSubops, err := a.w.DSSearchSubOperations.ScrapeData()
	multiErr = multierr.Append(multiErr, err)
	if err == nil {
		a.mb.RecordActiveDirectoryDsSuboperationRateDataPoint(now, searchSubops[0].Value, metadata.AttributeSuboperationType.Search)
	}

	threadsInUse, err := a.w.DSThreadsInUse.ScrapeData()
	multiErr = multierr.Append(multiErr, err)
	if err == nil {
		a.mb.RecordActiveDirectoryDsThreadCountDataPoint(now, int64(threadsInUse[0].Value))
	}

	ldapClientSessions, err := a.w.LDAPClientSessions.ScrapeData()
	multiErr = multierr.Append(multiErr, err)
	if err == nil {
		a.mb.RecordActiveDirectoryDsLdapClientSessionCountDataPoint(now, int64(ldapClientSessions[0].Value))
	}

	ldapBindTime, err := a.w.LDAPBindTime.ScrapeData()
	multiErr = multierr.Append(multiErr, err)
	if err == nil {
		a.mb.RecordActiveDirectoryDsLdapBindLastSuccessfulTimeDataPoint(now, int64(ldapBindTime[0].Value))
	}

	ldapSuccessfulBinds, err := a.w.LDAPSuccessfulBinds.ScrapeData()
	multiErr = multierr.Append(multiErr, err)
	if err == nil {
		a.mb.RecordActiveDirectoryDsLdapBindRateDataPoint(now, ldapSuccessfulBinds[0].Value)
	}

	ldapSearches, err := a.w.LDAPSearches.ScrapeData()
	multiErr = multierr.Append(multiErr, err)
	if err == nil {
		a.mb.RecordActiveDirectoryDsLdapSearchRateDataPoint(now, ldapSearches[0].Value)
	}

	if multiErr != nil {
		return pdata.Metrics(a.mb.Emit()), scrapererror.NewPartialScrapeError(multiErr, len(multierr.Errors(multiErr)))
	}

	return pdata.Metrics(a.mb.Emit()), nil
}

func (a *activeDirectoryDSScraper) shutdown(ctx context.Context) error {
	if a.w != nil {
		var err error

		err = multierr.Append(err, a.w.DRAInboundBytesCompressed.Close())
		err = multierr.Append(err, a.w.DRAInboundBytesNotCompressed.Close())
		err = multierr.Append(err, a.w.DRAOutboundBytesCompressed.Close())
		err = multierr.Append(err, a.w.DRAOutboundBytesNotCompressed.Close())
		err = multierr.Append(err, a.w.DRAInboundFullSyncObjectsRemaining.Close())
		err = multierr.Append(err, a.w.DRAInboundObjects.Close())
		err = multierr.Append(err, a.w.DRAOutboundObjects.Close())
		err = multierr.Append(err, a.w.DRAInboundProperties.Close())
		err = multierr.Append(err, a.w.DRAOutboundProperties.Close())
		err = multierr.Append(err, a.w.DRAInboundValuesDNs.Close())
		err = multierr.Append(err, a.w.DRAInboundValuesTotal.Close())
		err = multierr.Append(err, a.w.DRAOutboundValuesDNs.Close())
		err = multierr.Append(err, a.w.DRAOutboundValuesTotal.Close())
		err = multierr.Append(err, a.w.DRAPendingReplicationOperations.Close())
		err = multierr.Append(err, a.w.DRASyncFailuresSchemaMismatch.Close())
		err = multierr.Append(err, a.w.DRASyncRequestsSuccessful.Close())
		err = multierr.Append(err, a.w.DRASyncRequestsMade.Close())
		err = multierr.Append(err, a.w.DSDirectoryReads.Close())
		err = multierr.Append(err, a.w.DSDirectoryWrites.Close())
		err = multierr.Append(err, a.w.DSDirectorySearches.Close())
		err = multierr.Append(err, a.w.DSClientBinds.Close())
		err = multierr.Append(err, a.w.DSServerBinds.Close())
		err = multierr.Append(err, a.w.DSNameCacheHitRate.Close())
		err = multierr.Append(err, a.w.DSNotifyQueueSize.Close())
		err = multierr.Append(err, a.w.DSSecurityDescriptorPropagationsEvents.Close())
		err = multierr.Append(err, a.w.DSSearchSubOperations.Close())
		err = multierr.Append(err, a.w.DSSecurityDescripterSubOperations.Close())
		err = multierr.Append(err, a.w.DSThreadsInUse.Close())
		err = multierr.Append(err, a.w.LDAPClientSessions.Close())
		err = multierr.Append(err, a.w.LDAPBindTime.Close())
		err = multierr.Append(err, a.w.LDAPSuccessfulBinds.Close())
		err = multierr.Append(err, a.w.LDAPSearches.Close())

		a.w = nil
		return err
	}

	return nil
}
