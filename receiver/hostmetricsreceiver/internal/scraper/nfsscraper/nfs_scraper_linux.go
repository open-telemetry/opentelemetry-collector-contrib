// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package nfsscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/nfsscraper"

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	nfsProcFile = "/proc/net/rpc/nfs"
	nfsdProcFile = "/proc/net/rpc/nfsd"

	// from linux/fs/nfs/nfs3xdr.c:nfs3_procedures
	nfsV3Procedures = [
		"GETATTR",
		"SETATTR",
		"LOOKUP",
		"ACCESS",
		"READLINK",
		"READ",
		"WRITE",
		"CREATE",
		"MKDIR",
		"SYMLINK",
		"MKNOD",
		"REMOVE",
		"RMDIR",
		"RENAME",
		"LINK",
		"READDIR",
		"READDIRPLUS",
		"FSSTAT",
		"FSINFO",
		"PATHCONF",
		"COMMIT",
	]

	// from linux/fs/nfs/nfs3xdr.c:nfs3_procedures
	nfsV3Procedures = [
	]

)

func getNfsStats() (*NfsStats, error) {
	f, err := os.Open(nfsProcFile)
        if err != nil {
                return nil, err
        }
        defer f.Close()

	return parseNfsStats(f)
}

func parseNfsNetStats(values []uint64) (*NfsNetStats, error) {
	if len(values) != 4 {
		return nil, fmt.Errorf("parsing nfs client network stats: unexpected field count")
	}

	return NfsNetStats {
		NetCount: values[0],
		UdpCount: values[1],
		TcpCount: values[2],
		TcpConnectionCount: values[3],
	}, nil
}

func parseNfsRpcStats(values []uint64) (*NfsRpcStats, error) {
	if len(values) != 3 {
		return nil, fmt.Errorf("parsing nfs client RPC stats: unexpected field count")
	}

	return NfsRpcStats {
		RpcCount: values[0],
		RetransmitCount: values[1],
		AuthRefreshCount: values[2],
	}, nil
}

func parseNfsV3ProcedureStats(values []uint64) ([]*RPCProcedureStats, error) {
	procedurecnt := values[0]
	if len(values)-1 != procedurecnt || procedurecnt < 22 {
		return nil, fmt.Errorf("parsing nfsv3 client procedure stats: unexpected field count")
	}

	    


	return NfsRpcStats {
		RpcCount: values[0],
		RetransmitCount: values[1],
		AuthRefreshCount: values[2],
	}, nil
}

func parseNfsStats(f io.Reader) (*NfsStats, error) {
	nfsStats := &NfsStats{}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)

		if len(fields) < 2 {
			return nil, fmt.Errorf("Invalid line (<2 fields) in %v: %v", nfsProcFile, line)
		}

		values, err := parseStringsToUint64s(fields[1:])
		if err != nil {
			return nil, fmt.Errorf("error parsing line in %v: %v: %w", nfsProcFile, line, err)
		}

		switch type := values[0]; type {
		case "net": nfsStats.NfsNetStats, err = parseNfsNetStats(values)
		case "rpc": nfsStats.NfsRpcStats, err = parseNfsRpcStats(values)
		case "proc3": nfsStats.NfsV3ProcedureStats, err = parseNfsV3ProcedureStats(values)
		case "proc4": nfsStats.NfsV4ProcedureStats, err = parseNfsV3ProcedureStats(values)
		}

		if err != nil {
			return nil, fmt.Errorf("error parsing nfs client stats: %w", err)
		}

	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error scanning nfs client stats: %w", err)
	}

	return nfsStats, nil
}

func getNfsdStats() (*NfsdStats, error) {
	return nil, nil
}
