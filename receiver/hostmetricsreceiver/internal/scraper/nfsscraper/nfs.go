// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package nfsscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/nfsscraper"

// nfs.net.* stats
type nfsNetStats struct {
	netCount           uint64
	udpCount           uint64
	tcpCount           uint64
	tcpConnectionCount uint64
}

// nfs.rpc.* stats
type nfsRPCStats struct {
	rpcCount         uint64
	retransmitCount  uint64
	authRefreshCount uint64
}

// nfs.procedure.count / nfsd.procedure.count stats
// nfs.operation.count / nfsd.operation.count stats
type callStats struct {
	nfsVersion   int64
	nfsCallName  string
	nfsCallCount uint64
}

// nfsd.repcache.* stats
type nfsdRepcacheStats struct {
	hits    uint64
	misses  uint64
	nocache uint64
}

// nfsd.fh.* stats
// Note: linux/fs/nfsd/stats.c shows many deprecated (always 0) fh stats
type nfsdFhStats struct {
	stale uint64
}

// nfsd.io.* stats
type nfsdIoStats struct {
	read  uint64
	write uint64
}

// nfsd.thread.* stats
type nfsdThreadStats struct {
	threads uint64
}

// nfsd.net.* stats
type nfsdNetStats struct {
	netCount           uint64
	udpCount           uint64
	tcpCount           uint64
	tcpConnectionCount uint64
}

// nfsd.rpc.* stats
type nfsdRPCStats struct {
	rpcCount       uint64
	badCount       uint64
	badFmtCount    uint64
	badAuthCount   uint64
	badClientCount uint64
}

// 6 metrics + 22 NFSv3 procedures + 69 NFSv4 procedures = 97 metrics
type NfsStats struct {
	nfsNetStats         *nfsNetStats
	nfsRPCStats         *nfsRPCStats
	nfsV3ProcedureStats []callStats
	nfsV4ProcedureStats []callStats
	nfsV4OperationStats []callStats
}

// 15 metrics + 22 NFSv3 procedures + 76 NFSv4 procedures = 113 metrics
type nfsdStats struct {
	nfsdRepcacheStats    *nfsdRepcacheStats
	nfsdFhStats          *nfsdFhStats
	nfsdIoStats          *nfsdIoStats
	nfsdThreadStats      *nfsdThreadStats
	nfsdNetStats         *nfsdNetStats
	nfsdRPCStats         *nfsdRPCStats
	nfsdV3ProcedureStats []callStats
	nfsdV4ProcedureStats []callStats
	nfsdV4OperationStats []callStats
}
