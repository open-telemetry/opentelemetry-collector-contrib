// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package nfsscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/nfsscraper"

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

const (
	nfsProcFile  = "/proc/net/rpc/nfs"
	nfsdProcFile = "/proc/net/rpc/nfsd"
)

// from linux/fs/nfs/nfs3xdr.c:nfs3_procedures v6.12
// note that this array starts at element 1, with a NULL at element 0
var nfsV3Procedures = []string{
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

// from linux/fs/nfs/nfs4xdr.c:nfs4_procedures v6.12
// note that these are technically NFSv4 operations, part of a COMPOUND RPC procedure
// note that this array starts at element 1, with a NULL at element 0
var nfsV4Procedures = []string{
	"NULL",
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
var nfsdV4Operations = []string{
	"UNUSED_IGNORE0",
	"UNUSED_IGNORE1",
	"UNUSED_IGNORE2",
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

func getOSNfsStats() (*NfsStats, error) {
	/* for testing: (nfsProcFileOut from nfs_scraper_linux_test.go)
	data := strings.NewReader(nfsProcFileOut)

	rv, err := parseNfsStats(data)
	fmt.Fprintf(os.Stderr, "%#v\n", rv)
	return rv, err
	*/

	f, err := os.Open(nfsProcFile)
	if err != nil {
		return nil, err
	}

	defer f.Close()

	return parseNfsStats(f)
}

func getOSNfsdStats() (*nfsdStats, error) {
	/* for testing: (nfsdProcFileOut from nfs_scraper_linux_test.go)
	data := strings.NewReader(nfsdProcFileOut)

	rv, err := parseNfsdStats(data)
	fmt.Fprintf(os.Stderr, "%#v\n", rv)
	return rv, err
	*/

	f, err := os.Open(nfsdProcFile)
	if err != nil {
		return nil, err
	}

	defer f.Close()

	return parseNfsdStats(f)
}

func parseNfsNetStats(values []uint64) (*nfsNetStats, error) {
	if len(values) < 4 {
		return nil, errors.New("parsing nfs client network stats: unexpected field count")
	}

	return &nfsNetStats{
		netCount:           values[0],
		udpCount:           values[1],
		tcpCount:           values[2],
		tcpConnectionCount: values[3],
	}, nil
}

func parseNfsRPCStats(values []uint64) (*nfsRPCStats, error) {
	if len(values) < 3 {
		return nil, fmt.Errorf("parsing nfs client RPC stats: unexpected field count: %d", len(values))
	}

	return &nfsRPCStats{
		rpcCount:         values[0],
		retransmitCount:  values[1],
		authRefreshCount: values[2],
	}, nil
}

func parseNfsdNetStats(values []uint64) (*nfsdNetStats, error) {
	if len(values) < 4 {
		return nil, errors.New("parsing nfs server network stats: unexpected field count")
	}

	return &nfsdNetStats{
		netCount:           values[0],
		udpCount:           values[1],
		tcpCount:           values[2],
		tcpConnectionCount: values[3],
	}, nil
}

func parseNfsdRepcacheStats(values []uint64) (*nfsdRepcacheStats, error) {
	if len(values) < 3 {
		return nil, errors.New("parsing nfs server repcache stats: unexpected field count")
	}

	return &nfsdRepcacheStats{
		hits:    values[0],
		misses:  values[1],
		nocache: values[2],
	}, nil
}

func parseNfsdFhStats(values []uint64) (*nfsdFhStats, error) {
	if len(values) < 1 {
		return nil, errors.New("parsing nfs server fh stats: unexpected field count")
	}

	return &nfsdFhStats{
		stale: values[0],
	}, nil
}

func parseNfsdIoStats(values []uint64) (*nfsdIoStats, error) {
	if len(values) < 2 {
		return nil, errors.New("parsing nfs server io stats: unexpected field count")
	}

	return &nfsdIoStats{
		read:  values[0],
		write: values[1],
	}, nil
}

func parseNfsdThreadStats(values []uint64) (*nfsdThreadStats, error) {
	if len(values) < 1 {
		return nil, errors.New("parsing nfs server thread stats: unexpected field count")
	}

	return &nfsdThreadStats{
		threads: values[0],
	}, nil
}

func parseNfsdRPCStats(values []uint64) (*nfsdRPCStats, error) {
	if len(values) < 5 {
		return nil, fmt.Errorf("parsing nfs client RPC stats: unexpected field count: %d", len(values))
	}

	return &nfsdRPCStats{
		rpcCount:       values[0],
		badCount:       values[1],
		badFmtCount:    values[2],
		badAuthCount:   values[3],
		badClientCount: values[4],
	}, nil
}

func parseNfsCallStats(nfsVersion int64, names []string, values []uint64) ([]callStats, error) {
	if len(values) < 2 {
		return nil, errors.New("found empty stats line")
	}

	stats := make([]callStats, len(values)-1)
	numCalls := values[0]

	if len(values)-1 != int(numCalls) {
		return nil, errors.New("parsing nfs stats: unexpected field count")
	}

	for i, calls := range values {
		if i == 0 {
			continue // first element is numCalls
		}

		if i-1 > len(names)-1 {
			// found yet-to-be-supported procedures
			break
		}

		stats[i-1].nfsVersion = nfsVersion
		stats[i-1].nfsCallName = names[i-1]
		stats[i-1].nfsCallCount = calls
	}

	return stats, nil
}

func parseNfsStats(f io.Reader) (*NfsStats, error) {
	nfsStats := &NfsStats{}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)

		debugLine("/proc/net/rpc/nfs", line)

		if len(fields) < 2 {
			return nil, fmt.Errorf("Invalid line (<2 fields) in %v: %v", nfsProcFile, line)
		}

		var parse func([]uint64) error
		switch stattype := fields[0]; stattype {
		case "net":
			parse = func(values []uint64) error {
				var err error
				nfsStats.nfsNetStats, err = parseNfsNetStats(values)
				return err
			}
		case "rpc":
			parse = func(values []uint64) error {
				var err error
				nfsStats.nfsRPCStats, err = parseNfsRPCStats(values)
				return err
			}
		case "proc3":
			parse = func(values []uint64) error {
				var err error
				nfsStats.nfsV3ProcedureStats, err = parseNfsCallStats(3, nfsV3Procedures, values)
				return err
			}
		case "proc4":
			parse = func(values []uint64) error {
				var err error
				// Linux kernel calls NFSv4 client operations procedures, but they're actually
				// operations of compound procedures, per RFC7530
				nfsStats.nfsV4OperationStats, err = parseNfsCallStats(4, nfsV4Procedures, values)
				return err
			}
		}

		if parse != nil {
			values, err := parseStringsToUint64s(fields[1:])
			if err != nil {
				return nil, fmt.Errorf("error parsing line to Uint64 in %v: %v: %w", nfsdProcFile, line, err)
			}

			err = parse(values)
			if err != nil {
				return nil, fmt.Errorf("error parsing line in %v: %v: %w", nfsdProcFile, line, err)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error scanning nfs client stats: %w", err)
	}

	return nfsStats, nil
}

func parseNfsdStats(f io.Reader) (*nfsdStats, error) {
	nfsdStats := &nfsdStats{}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)

		debugLine("/proc/net/rpc/nfsd", line)

		if len(fields) < 2 {
			return nil, fmt.Errorf("Invalid line (<2 fields) in %v: %v", nfsdProcFile, line)
		}

		var parse func([]uint64) error
		switch stattype := fields[0]; stattype {
		case "rc":
			parse = func(values []uint64) error {
				var err error
				nfsdStats.nfsdRepcacheStats, err = parseNfsdRepcacheStats(values)
				return err
			}
		case "fh":
			parse = func(values []uint64) error {
				var err error
				nfsdStats.nfsdFhStats, err = parseNfsdFhStats(values)
				return err
			}
		case "io":
			parse = func(values []uint64) error {
				var err error
				nfsdStats.nfsdIoStats, err = parseNfsdIoStats(values)
				return err
			}
		case "th":
			parse = func(values []uint64) error {
				var err error
				nfsdStats.nfsdThreadStats, err = parseNfsdThreadStats(values)
				return err
			}
		case "net":
			parse = func(values []uint64) error {
				var err error
				nfsdStats.nfsdNetStats, err = parseNfsdNetStats(values)
				return err
			}
		case "rpc":
			parse = func(values []uint64) error {
				var err error
				nfsdStats.nfsdRPCStats, err = parseNfsdRPCStats(values)
				return err
			}
		case "proc3":
			parse = func(values []uint64) error {
				var err error
				nfsdStats.nfsdV3ProcedureStats, err = parseNfsCallStats(3, nfsdV3Procedures, values)
				return err
			}
		case "proc4":
			parse = func(values []uint64) error {
				var err error
				nfsdStats.nfsdV4ProcedureStats, err = parseNfsCallStats(4, nfsdV4Procedures, values)
				return err
			}
		case "proc4ops":
			parse = func(values []uint64) error {
				var err error
				nfsdStats.nfsdV4OperationStats, err = parseNfsCallStats(4, nfsdV4Operations, values)
				return err
			}
		}

		if parse != nil {
			var err error
			var values []uint64

			if fields[0] == "th" {
				// th has a mix of uint64 and double. the double values are obsolete (always 0.000)
				values, err = parseStringsToUint64s(fields[1:2])
			} else {
				values, err = parseStringsToUint64s(fields[1:])
			}

			if err != nil {
				return nil, fmt.Errorf("error parsing line to Uint64 in %v: %v: %w", nfsdProcFile, line, err)
			}

			err = parse(values)
			if err != nil {
				return nil, fmt.Errorf("error parsing line in %v: %v: %w", nfsdProcFile, line, err)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error scanning nfs server stats: %w", err)
	}

	return nfsdStats, nil
}

// parseStringsToUint64s parses a slice of strings into a slice of uint64
func parseStringsToUint64s(strSlice []string) ([]uint64, error) {
	uint64Slice := make([]uint64, 0, len(strSlice))

	for _, s := range strSlice {
		val, err := strconv.ParseUint(s, 10, 64) // Assuming decimal base and uint64
		if err != nil {
			return nil, fmt.Errorf("failed to convert string '%s' to uint64: %w", s, err)
		}
		uint64Slice = append(uint64Slice, val)
	}

	return uint64Slice, nil
}

func CanScrapeAll() bool {
	_, err := os.Stat(nfsProcFile)
	if err == nil {
		_, err := os.Stat(nfsdProcFile)
		if err == nil {
			return true
		}
	}

	return false
}
