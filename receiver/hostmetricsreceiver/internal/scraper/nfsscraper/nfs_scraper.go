// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package nfsscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/nfsscraper"

import (
	"context"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/nfsscraper/internal/metadata"
)

var (
	// 3 net metrics + 3 rpc metrics + len(nfs_scraper_linux.go:nfsV3Procedures) + len(nfs_scraper_linux.go:nfsV4Procedures) = 97 metrics
	nfsMetricsLen = 97
	// 3 repcache metrics + 1 fh metric + 2 io metrics + 1 thread metric + 3 net metrics + 5 rpc metrics + len(nfs_scraper_linux.go:nfsdV3Procedures) + len(nfs_scraper_linux.go:nfsdV4Procedures) + len(nfs_scraper_linux.go:nfsdV4Operations) = 115 metrics
	nfsdMetricsLen = 115
)

// nfsScraper for NFS Metrics
type nfsScraper struct {
	settings scraper.Settings
	config   *Config
	mb       *metadata.MetricsBuilder

	getNfsStats  func() (*NfsStats, error)
	getNfsdStats func() (*nfsdStats, error)

	nfsStats  *NfsStats
	nfsdStats *nfsdStats
}

// newNfsScraper creates a metric scraper for NFS metrics
func newNfsScraper(settings scraper.Settings, cfg *Config) *nfsScraper {
	return &nfsScraper{
		settings:     settings,
		config:       cfg,
		getNfsStats:  getOSNfsStats,
		getNfsdStats: getOSNfsdStats,
	}
}

func (s *nfsScraper) start(context.Context, component.Host) error {
	s.mb = metadata.NewMetricsBuilder(s.config.MetricsBuilderConfig, s.settings)
	return nil
}

func (s *nfsScraper) scrape(context.Context) (pmetric.Metrics, error) {
	var err error
	var errs scrapererror.ScrapeErrors
	now := pcommon.NewTimestampFromTime(time.Now())

	s.nfsStats, err = s.getNfsStats()
	if err == nil {
		s.recordNfsMetrics(now)
	} else {
		errs.AddPartial(nfsMetricsLen, err)
	}

	s.nfsdStats, err = s.getNfsdStats()
	if err == nil {
		s.recordNfsdMetrics(now)
	} else {
		errs.AddPartial(nfsdMetricsLen, err)
	}

	return s.mb.Emit(), errs.Combine()
}

func (s *nfsScraper) recordNfsMetrics(now pcommon.Timestamp) {
	if s.nfsStats == nil {
		return
	}

	if s.nfsStats.nfsNetStats != nil {
		s.mb.RecordNfsClientNetCountDataPoint(now, int64(s.nfsStats.nfsNetStats.udpCount), metadata.AttributeNetworkTransportUdp)
		s.mb.RecordNfsClientNetCountDataPoint(now, int64(s.nfsStats.nfsNetStats.tcpCount), metadata.AttributeNetworkTransportTcp)
		s.mb.RecordNfsClientNetTCPConnectionAcceptedDataPoint(now, int64(s.nfsStats.nfsNetStats.tcpConnectionCount))
	}

	if s.nfsStats.nfsRPCStats != nil {
		s.mb.RecordNfsClientRPCCountDataPoint(now, int64(s.nfsStats.nfsRPCStats.rpcCount))
		s.mb.RecordNfsClientRPCRetransmitCountDataPoint(now, int64(s.nfsStats.nfsRPCStats.retransmitCount))
		s.mb.RecordNfsClientRPCAuthrefreshCountDataPoint(now, int64(s.nfsStats.nfsRPCStats.authRefreshCount))
	}

	if s.nfsStats.nfsV3ProcedureStats != nil {
		for _, callStat := range s.nfsStats.nfsV3ProcedureStats {
			s.mb.RecordNfsClientProcedureCountDataPoint(now, int64(callStat.nfsCallCount), callStat.nfsVersion, callStat.nfsCallName)
		}
	}

	if s.nfsStats.nfsV4ProcedureStats != nil {
		for _, callStat := range s.nfsStats.nfsV4ProcedureStats {
			s.mb.RecordNfsClientProcedureCountDataPoint(now, int64(callStat.nfsCallCount), callStat.nfsVersion, callStat.nfsCallName)
		}
	}

	if s.nfsStats.nfsV4OperationStats != nil {
		for _, callStat := range s.nfsStats.nfsV4OperationStats {
			s.mb.RecordNfsClientOperationCountDataPoint(now, int64(callStat.nfsCallCount), callStat.nfsVersion, callStat.nfsCallName)
		}
	}
}

func (s *nfsScraper) recordNfsdMetrics(now pcommon.Timestamp) {
	if s.nfsdStats == nil {
		return
	}

	if s.nfsdStats.nfsdRepcacheStats != nil {
		s.mb.RecordNfsServerRepcacheRequestsDataPoint(now, int64(s.nfsdStats.nfsdRepcacheStats.hits), metadata.AttributeNfsServerRepcacheStatusHit)
		s.mb.RecordNfsServerRepcacheRequestsDataPoint(now, int64(s.nfsdStats.nfsdRepcacheStats.misses), metadata.AttributeNfsServerRepcacheStatusMiss)
		s.mb.RecordNfsServerRepcacheRequestsDataPoint(now, int64(s.nfsdStats.nfsdRepcacheStats.nocache), metadata.AttributeNfsServerRepcacheStatusNocache)
	}

	if s.nfsdStats.nfsdFhStats != nil {
		s.mb.RecordNfsServerFhStaleCountDataPoint(now, int64(s.nfsdStats.nfsdFhStats.stale))
	}

	if s.nfsdStats.nfsdIoStats != nil {
		s.mb.RecordNfsServerIoDataPoint(now, int64(s.nfsdStats.nfsdIoStats.read), metadata.AttributeNetworkIoDirectionReceive)
		s.mb.RecordNfsServerIoDataPoint(now, int64(s.nfsdStats.nfsdIoStats.write), metadata.AttributeNetworkIoDirectionTransmit)
	}

	if s.nfsdStats.nfsdThreadStats != nil {
		s.mb.RecordNfsServerThreadCountDataPoint(now, int64(s.nfsdStats.nfsdThreadStats.threads))
	}

	if s.nfsdStats.nfsdNetStats != nil {
		s.mb.RecordNfsServerNetCountDataPoint(now, int64(s.nfsdStats.nfsdNetStats.udpCount), metadata.AttributeNetworkTransportUdp)
		s.mb.RecordNfsServerNetCountDataPoint(now, int64(s.nfsdStats.nfsdNetStats.tcpCount), metadata.AttributeNetworkTransportTcp)
		s.mb.RecordNfsServerNetTCPConnectionAcceptedDataPoint(now, int64(s.nfsdStats.nfsdNetStats.tcpConnectionCount))
	}

	if s.nfsdStats.nfsdRPCStats != nil {
		s.mb.RecordNfsServerRPCCountDataPoint(now, int64(s.nfsdStats.nfsdRPCStats.badFmtCount), metadata.AttributeErrorTypeFormat)
		s.mb.RecordNfsServerRPCCountDataPoint(now, int64(s.nfsdStats.nfsdRPCStats.badAuthCount), metadata.AttributeErrorTypeAuth)
		s.mb.RecordNfsServerRPCCountDataPoint(now, int64(s.nfsdStats.nfsdRPCStats.badClientCount), metadata.AttributeErrorTypeClient)
	}

	if s.nfsdStats.nfsdV3ProcedureStats != nil {
		for _, callStat := range s.nfsdStats.nfsdV3ProcedureStats {
			s.mb.RecordNfsServerProcedureCountDataPoint(now, int64(callStat.nfsCallCount), callStat.nfsVersion, callStat.nfsCallName)
		}
	}

	if s.nfsdStats.nfsdV4ProcedureStats != nil {
		for _, callStat := range s.nfsdStats.nfsdV4ProcedureStats {
			s.mb.RecordNfsServerProcedureCountDataPoint(now, int64(callStat.nfsCallCount), callStat.nfsVersion, callStat.nfsCallName)
		}
	}

	if s.nfsdStats.nfsdV4OperationStats != nil {
		for _, callStat := range s.nfsdStats.nfsdV4OperationStats {
			if !strings.HasPrefix(callStat.nfsCallName, "UNUSED_IGNORE") {
				s.mb.RecordNfsServerOperationCountDataPoint(now, int64(callStat.nfsCallCount), callStat.nfsVersion, callStat.nfsCallName)
			}
		}
	}
}
