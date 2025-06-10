// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package nfsscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/nfsscraper"

// nfs.net.* stats
type NfsNetStats struct {
	NetCount           uint64
	UDPCount           uint64
	TCPCount           uint64
	TCPConnectionCount uint64
}

// nfs.rpc.* stats
type NfsRPCStats struct {
	RPCCount         uint64
	RetransmitCount  uint64
	AuthRefreshCount uint64
}

// nfs.procedure.count / nfsd.procedure.count stats
// nfs.operation.count / nfsd.operation.count stats
type CallStats struct {
	NFSVersion   int64
	NFSCallName  string
	NFSCallCount uint64
}

// nfsd.repcache.* stats
type NfsdRepcacheStats struct {
	Hits    uint64
	Misses  uint64
	Nocache uint64
}

// nfsd.fh.* stats
// Note: linux/fs/nfsd/stats.c shows many deprecated (always 0) fh stats
type NfsdFhStats struct {
	Stale uint64
}

// nfsd.io.* stats
type NfsdIoStats struct {
	Read  uint64
	Write uint64
}

// nfsd.thread.* stats
type NfsdThreadStats struct {
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
	NfsNetStats         *NfsNetStats
	NfsRPCStats         *NfsRPCStats
	NfsV3ProcedureStats *[]CallStats
	NfsV4ProcedureStats *[]CallStats
	NfsV4OperationStats *[]CallStats
}

// 15 metrics + 22 NFSv3 procedures + 76 NFSv4 procedures = 113 metrics
type NfsdStats struct {
	NfsdRepcacheStats    *NfsdRepcacheStats
	NfsdFhStats          *NfsdFhStats
	NfsdIoStats          *NfsdIoStats
	NfsdThreadStats      *NfsdThreadStats
	NfsdNetStats         *NfsdNetStats
	NfsdRPCStats         *NfsdRPCStats
	NfsdV3ProcedureStats *[]CallStats
	NfsdV4ProcedureStats *[]CallStats
	NfsdV4OperationStats *[]CallStats
}
