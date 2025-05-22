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
)

// from linux/fs/nfs/nfs3xdr.c:nfs3_procedures v6.12
var nfsV3Procedures = []string{
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
}

// from linux/fs/nfs/nfs4xdr.c:nfs4_procedures v6.12
var nfsV4Procedures = []string{
	"READ",
	"WRITE",
	"COMMIT",
	"OPEN",
	"OPEN_CONFIRM",
	"OPEN_NOATTR",
	"OPEN_DOWNGRADE",
	"CLOSE",
	"SETATTR",
	"FSINFO",
	"RENEW",
	"SETCLIENTID",
	"SETCLIENTID_CONFIRM",
	"LOCK",
	"LOCKT",
	"LOCKU",
	"ACCESS",
	"GETATTR",
	"LOOKUP",
	"LOOKUP_ROOT",
	"REMOVE",
	"RENAME",
	"LINK",
	"SYMLINK",
	"CREATE",
	"PATHCONF",
	"STATFS",
	"READLINK",
	"READDIR",
	"SERVER_CAPS",
	"DELEGRETURN",
	"GETACL",
	"SETACL",
	"FS_LOCATIONS",
	"RELEASE_LOCKOWNER",
	"SECINFO",
	"FSID_PRESENT",
	"EXCHANGE_ID",
	"CREATE_SESSION",
	"DESTROY_SESSION",
	"SEQUENCE",
	"GET_LEASE_TIME",
	"RECLAIM_COMPLETE",
	"GETDEVICEINFO",
	"LAYOUTGET",
	"LAYOUTCOMMIT",
	"LAYOUTRETURN",
	"SECINFO_NO_NAME",
	"TEST_STATEID",
	"FREE_STATEID",
	"GETDEVICELIST",
	"BIND_CONN_TO_SESSION",
	"DESTROY_CLIENTID",
	"SEEK",
	"ALLOCATE",
	"DEALLOCATE",
	"LAYOUTSTATS",
	"CLONE",
	"COPY",
	"OFFLOAD_CANCEL",
	"COPY_NOTIFY",
	"LOOKUPP",
	"LAYOUTERROR",
	"GETXATTR",
	"SETXATTR",
	"LISTXATTRS",
	"REMOVEXATTR",
	"READ_PLUS",
}

// from linux/fs/nfsd/nfs3proc.c:nfsd_procedures3 v6.12
var nfsdV3Procedures = []string{
	"NULL",
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
}

// from linux/fs/nfsd/nfs4proc.c:nfsd_procedures4 v6.12
var nfsdV4Procedures = []string{
	"NULL",
	"COMPOUND",
}

// from linux/include/linux/nfs4.h:nfs_opnum4 v6.12
var nfsdV4Ops = []string{
	"UNUSED0",
	"UNUSED1",
	"UNUSED2",
	"ACCESS",
	"CLOSE",
	"COMMIT",
	"CREATE",
	"DELEGPURGE",
	"DELEGRETURN",
	"GETATTR",
	"GETFH",
	"LINK",
	"LOCK",
	"LOCKT",
	"LOCKU",
	"LOOKUP",
	"LOOKUPP",
	"NVERIFY",
	"OPEN",
	"OPENATTR",
	"OPEN_CONFIRM",
	"OPEN_DOWNGRADE",
	"PUTFH",
	"PUTPUBFH",
	"PUTROOTFH",
	"READ",
	"READDIR",
	"READLINK",
	"REMOVE",
	"RENAME",
	"RENEW",
	"RESTOREFH",
	"SAVEFH",
	"SECINFO",
	"SETATTR",
	"SETCLIENTID",
	"SETCLIENTID_CONFIRM",
	"VERIFY",
	"WRITE",
	"RELEASE_LOCKOWNER",
	"BACKCHANNEL_CTL",
	"BIND_CONN_TO_SESSION",
	"EXCHANGE_ID",
	"CREATE_SESSION",
	"DESTROY_SESSION",
	"FREE_STATEID",
	"GET_DIR_DELEGATION",
	"GETDEVICEINFO",
	"GETDEVICELIST",
	"LAYOUTCOMMIT",
	"LAYOUTGET",
	"LAYOUTRETURN",
	"SECINFO_NO_NAME",
	"SEQUENCE",
	"SET_SSV",
	"TEST_STATEID",
	"WANT_DELEGATION",
	"DESTROY_CLIENTID",
	"RECLAIM_COMPLETE",
	"ALLOCATE",
	"COPY",
	"COPY_NOTIFY",
	"DEALLOCATE",
	"IO_ADVISE",
	"LAYOUTERROR",
	"LAYOUTSTATS",
	"OFFLOAD_CANCEL",
	"OFFLOAD_STATUS",
	"READ_PLUS",
	"SEEK",
	"WRITE_SAME",
	"CLONE",
	"GETXATTR",
	"SETXATTR",
	"LISTXATTRS",
	"REMOVEXATTR",
}


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

	return &NfsNetStats {
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

	return &NfsRpcStats {
		RpcCount: values[0],
		RetransmitCount: values[1],
		AuthRefreshCount: values[2],
	}, nil
}

func parseNfsV3ProcedureStats(values []uint64) ([]*RPCProcedureStats, error) {
	procedurecnt := values[0]
	
	if len(values)-1 != int(procedurecnt) || procedurecnt < 22 {
		return nil, fmt.Errorf("parsing nfsv3 client procedure stats: unexpected field count")
	}

	return nil, nil
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

		switch stattype := fields[0]; stattype {
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
