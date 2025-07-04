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
	getNfsdStats func() (*NfsdStats, error)

	nfsStats  *NfsStats
	nfsdStats *NfsdStats
}

// newNfsScraper creates an NFS Scraper related metric
func newNfsScraper(settings scraper.Settings, cfg *Config) *nfsScraper {
	return &nfsScraper{
		settings:     settings,
		config:       cfg,
		getNfsStats:  getOSNfsStats,
		getNfsdStats: getOSNfsdStats,
		nfsStats:     nil,
		nfsdStats:    nil,
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
		s.recordNfsMetrics(now, s.nfsStats)
	} else {
		errs.AddPartial(nfsMetricsLen, err)
	}

	s.nfsdStats, err = s.getNfsdStats()
	if err == nil {
		s.recordNfsdMetrics(now, s.nfsdStats)
	} else {
		errs.AddPartial(nfsdMetricsLen, err)
	}

	return s.mb.Emit(), errs.Combine()
}

func (s *nfsScraper) recordNfsMetrics(now pcommon.Timestamp, nfsStats *NfsStats) {
	if nfsStats.NfsNetStats != nil {
		s.mb.RecordNfsClientNetCountDataPoint(now, int64(nfsStats.NfsNetStats.NetCount))
		s.mb.RecordNfsClientNetUDPCountDataPoint(now, int64(nfsStats.NfsNetStats.UDPCount))
		s.mb.RecordNfsClientNetTCPCountDataPoint(now, int64(nfsStats.NfsNetStats.TCPCount))
		s.mb.RecordNfsClientNetTCPConnectionCountDataPoint(now, int64(nfsStats.NfsNetStats.TCPConnectionCount))
	}

	if nfsStats.NfsRPCStats != nil {
		s.mb.RecordNfsClientRPCCountDataPoint(now, int64(nfsStats.NfsRPCStats.RPCCount))
		s.mb.RecordNfsClientRPCRetransmitCountDataPoint(now, int64(nfsStats.NfsRPCStats.RetransmitCount))
		s.mb.RecordNfsClientRPCAuthrefreshCountDataPoint(now, int64(nfsStats.NfsRPCStats.AuthRefreshCount))
	}

	if nfsStats.NfsV3ProcedureStats != nil {
		for _, callStat := range *nfsStats.NfsV3ProcedureStats {
			s.mb.RecordNfsClientProcedureCountDataPoint(now, int64(callStat.NFSCallCount), callStat.NFSVersion, callStat.NFSCallName)
		}
	}

	if nfsStats.NfsV4ProcedureStats != nil {
		for _, callStat := range *nfsStats.NfsV4ProcedureStats {
			s.mb.RecordNfsClientProcedureCountDataPoint(now, int64(callStat.NFSCallCount), callStat.NFSVersion, callStat.NFSCallName)
		}
	}

	if nfsStats.NfsV4OperationStats != nil {
		for _, callStat := range *nfsStats.NfsV4OperationStats {
			s.mb.RecordNfsClientOperationCountDataPoint(now, int64(callStat.NFSCallCount), callStat.NFSVersion, callStat.NFSCallName)
		}
	}
}

func (s *nfsScraper) recordNfsdMetrics(now pcommon.Timestamp, nfsdStats *NfsdStats) {
	if nfsdStats.NfsdRepcacheStats != nil {
		s.mb.RecordNfsServerRepcacheHitsDataPoint(now, int64(nfsdStats.NfsdRepcacheStats.Hits))
		s.mb.RecordNfsServerRepcacheMissesDataPoint(now, int64(nfsdStats.NfsdRepcacheStats.Misses))
		s.mb.RecordNfsServerRepcacheNocacheDataPoint(now, int64(nfsdStats.NfsdRepcacheStats.Nocache))
	}

	if nfsdStats.NfsdFhStats != nil {
		s.mb.RecordNfsServerFhStaleCountDataPoint(now, int64(nfsdStats.NfsdFhStats.Stale))
	}

	if nfsdStats.NfsdIoStats != nil {
		s.mb.RecordNfsServerIoReadCountDataPoint(now, int64(nfsdStats.NfsdIoStats.Read))
		s.mb.RecordNfsServerIoWriteCountDataPoint(now, int64(nfsdStats.NfsdIoStats.Write))
	}

	if nfsdStats.NfsdThreadStats != nil {
		s.mb.RecordNfsServerThreadCountDataPoint(now, int64(nfsdStats.NfsdThreadStats.Threads))
	}

	if nfsdStats.NfsdNetStats != nil {
		s.mb.RecordNfsServerNetCountDataPoint(now, int64(nfsdStats.NfsdNetStats.NetCount))
		s.mb.RecordNfsServerNetUDPCountDataPoint(now, int64(nfsdStats.NfsdNetStats.UDPCount))
		s.mb.RecordNfsServerNetTCPCountDataPoint(now, int64(nfsdStats.NfsdNetStats.TCPCount))
		s.mb.RecordNfsServerNetTCPConnectionCountDataPoint(now, int64(nfsdStats.NfsdNetStats.TCPConnectionCount))
	}

	if nfsdStats.NfsdRPCStats != nil {
		s.mb.RecordNfsServerRPCCountDataPoint(now, int64(nfsdStats.NfsdRPCStats.RPCCount))
		s.mb.RecordNfsServerRPCBadCountDataPoint(now, int64(nfsdStats.NfsdRPCStats.BadCount))
		s.mb.RecordNfsServerRPCBadfmtCountDataPoint(now, int64(nfsdStats.NfsdRPCStats.BadFmtCount))
		s.mb.RecordNfsServerRPCBadauthCountDataPoint(now, int64(nfsdStats.NfsdRPCStats.BadAuthCount))
		s.mb.RecordNfsServerRPCBadclientCountDataPoint(now, int64(nfsdStats.NfsdRPCStats.BadClientCount))
	}

	if nfsdStats.NfsdV3ProcedureStats != nil {
		for _, callStat := range *nfsdStats.NfsdV3ProcedureStats {
			s.mb.RecordNfsServerProcedureCountDataPoint(now, int64(callStat.NFSCallCount), callStat.NFSVersion, callStat.NFSCallName)
		}
	}

	if nfsdStats.NfsdV4ProcedureStats != nil {
		for _, callStat := range *nfsdStats.NfsdV4ProcedureStats {
			s.mb.RecordNfsServerProcedureCountDataPoint(now, int64(callStat.NFSCallCount), callStat.NFSVersion, callStat.NFSCallName)
		}
	}

	if nfsdStats.NfsdV4OperationStats != nil {
		for _, callStat := range *nfsdStats.NfsdV4OperationStats {
			s.mb.RecordNfsServerOperationCountDataPoint(now, int64(callStat.NFSCallCount), callStat.NFSVersion, callStat.NFSCallName)
		}
	}
}
