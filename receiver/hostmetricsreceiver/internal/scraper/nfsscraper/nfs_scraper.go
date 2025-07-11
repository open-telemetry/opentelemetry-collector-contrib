// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package nfsscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/nfsscraper"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/nfsscraper/internal/metadata"
)

const (
	nfsMetricsLen  = 97
	nfsdMetricsLen = 113
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
		getNfsdStats: getOSnfsdStats,
	}
}

func (s *nfsScraper) start(_ context.Context, _ component.Host) error {
	s.mb = metadata.NewMetricsBuilder(s.config.MetricsBuilderConfig, s.settings)
	return nil
}

func (s *nfsScraper) scrape(_ context.Context) (pmetric.Metrics, error) {
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
		s.mb.RecordNfsClientNetCountDataPoint(now, int64(s.nfsStats.nfsNetStats.netCount))
		s.mb.RecordNfsClientNetUDPCountDataPoint(now, int64(s.nfsStats.nfsNetStats.udpCount))
		s.mb.RecordNfsClientNetTCPCountDataPoint(now, int64(s.nfsStats.nfsNetStats.tcpCount))
		s.mb.RecordNfsClientNetTCPConnectionCountDataPoint(now, int64(s.nfsStats.nfsNetStats.tcpConnectionCount))
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
		s.mb.RecordNfsServerRepcacheHitsDataPoint(now, int64(s.nfsdStats.nfsdRepcacheStats.hits))
		s.mb.RecordNfsServerRepcacheMissesDataPoint(now, int64(s.nfsdStats.nfsdRepcacheStats.misses))
		s.mb.RecordNfsServerRepcacheNocacheDataPoint(now, int64(s.nfsdStats.nfsdRepcacheStats.nocache))
	}

	if s.nfsdStats.nfsdFhStats != nil {
		s.mb.RecordNfsServerFhStaleCountDataPoint(now, int64(s.nfsdStats.nfsdFhStats.stale))
	}

	if s.nfsdStats.nfsdIoStats != nil {
		s.mb.RecordNfsServerIoReadCountDataPoint(now, int64(s.nfsdStats.nfsdIoStats.read))
		s.mb.RecordNfsServerIoWriteCountDataPoint(now, int64(s.nfsdStats.nfsdIoStats.write))
	}

	if s.nfsdStats.nfsdThreadStats != nil {
		s.mb.RecordNfsServerThreadCountDataPoint(now, int64(s.nfsdStats.nfsdThreadStats.threads))
	}

	if s.nfsdStats.nfsdNetStats != nil {
		s.mb.RecordNfsServerNetCountDataPoint(now, int64(s.nfsdStats.nfsdNetStats.netCount))
		s.mb.RecordNfsServerNetUDPCountDataPoint(now, int64(s.nfsdStats.nfsdNetStats.udpCount))
		s.mb.RecordNfsServerNetTCPCountDataPoint(now, int64(s.nfsdStats.nfsdNetStats.tcpCount))
		s.mb.RecordNfsServerNetTCPConnectionCountDataPoint(now, int64(s.nfsdStats.nfsdNetStats.tcpConnectionCount))
	}

	if s.nfsdStats.nfsdRPCStats != nil {
		s.mb.RecordNfsServerRPCCountDataPoint(now, int64(s.nfsdStats.nfsdRPCStats.rpcCount))
		s.mb.RecordNfsServerRPCBadCountDataPoint(now, int64(s.nfsdStats.nfsdRPCStats.badCount))
		s.mb.RecordNfsServerRPCBadfmtCountDataPoint(now, int64(s.nfsdStats.nfsdRPCStats.badFmtCount))
		s.mb.RecordNfsServerRPCBadauthCountDataPoint(now, int64(s.nfsdStats.nfsdRPCStats.badAuthCount))
		s.mb.RecordNfsServerRPCBadclientCountDataPoint(now, int64(s.nfsdStats.nfsdRPCStats.badClientCount))
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
			s.mb.RecordNfsServerOperationCountDataPoint(now, int64(callStat.nfsCallCount), callStat.nfsVersion, callStat.nfsCallName)
		}
	}
}
