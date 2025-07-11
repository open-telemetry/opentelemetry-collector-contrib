// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package nfsscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/nfsscraper"

// nfs.net.* stats
type nfsNetStats struct {
	NetCount           uint64
	UDPCount           uint64
	TCPCount           uint64
	TCPConnectionCount uint64
}

// nfs.rpc.* stats
type nfsRPCStats struct {
	RPCCount         uint64
	RetransmitCount  uint64
	AuthRefreshCount uint64
}

// nfs.procedure.count / nfsd.procedure.count stats
// nfs.operation.count / nfsd.operation.count stats
type callStats struct {
	NFSVersion   int64
	NFSCallName  string
	NFSCallCount uint64
}

// nfsd.repcache.* stats
type nfsdRepcacheStats struct {
	Hits    uint64
	Misses  uint64
	Nocache uint64
}

// nfsd.fh.* stats
// Note: linux/fs/nfsd/stats.c shows many deprecated (always 0) fh stats
type nfsdFhStats struct {
	Stale uint64
}

// nfsd.io.* stats
type nfsdIoStats struct {
	Read  uint64
	Write uint64
}

// nfsd.thread.* stats
type nfsdThreadStats struct {
	Threads uint64
}

// nfsd.net.* stats
type NfsdNetStats struct {
	NetCount           uint64
	UDPCount           uint64
	TCPCount           uint64
	TCPConnectionCount uint64
}

// nfsd.rpc.* stats
type NfsdRPCStats struct {
	RPCCount       uint64
	BadCount       uint64
	BadFmtCount    uint64
	BadAuthCount   uint64
	BadClientCount uint64
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
	NfsdNetStats         *NfsdNetStats
	NfsdRPCStats         *NfsdRPCStats
	NfsdV3ProcedureStats []callStats
	NfsdV4ProcedureStats []callStats
	NfsdV4OperationStats []callStats
}
