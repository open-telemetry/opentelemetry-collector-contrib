// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package nfsscraper

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	nfsProcFileOut = `net 8 843 666 157
rpc 220 789 662
proc3 22 191 360 118 397 455 77 581 182 333 767 235 558 371 872 877 652 265 463 416 200 942 235
proc4 69 32 829 218 185 789 767 813 512 615 355 207 211 992 234 736 629 862 860 117 752 128 200 494 372 158 420 757 783 46 725 180 301 305 856 965 416 653 340 500 650 545 155 620 354 959 965 986 526 367 388 373 786 890 459 810 679 939 583 790 333 455 115 155 884 571 409 540 293 721
`

	nfsdProcFileOut = `rc 795 819 351
fh 709 9 178 461 14
io 111 464
th 261 966 860 728 589 845 366 825 913 257 789 77
ra 579 364 872 902 542 886 245 835 517 437 593 152
net 1 43 26 597
rpc 872 367 960 94 748
proc3 22 124 554 529 64 928 316 531 43 724 822 237 665 620 22 335 137 236 222 658 654 209 382
proc4 2 512 878
proc4ops 76 725 607 978 86 442 878 262 489 962 909 563 468 722 104 47 214 305 564 776 373 444 6 265 163 397 817 73 90 630 664 984 981 502 682 210 639 484 924 337 857 667 984 498 76 515 657 596 31 781 437 23 846 867 241 648 169 64 151 447 848 625 185 586 890 446 317 503 32 935 459 386 291 817 74 592 562
wdeleg_getattr 901
`
)

func mockGetOSNfsStats() (*NfsStats, error) {
	data := strings.NewReader(nfsProcFileOut)

	return parseNfsStats(data)
}

func mockGetOSnfsdStats() (*nfsdStats, error) {
	data := strings.NewReader(nfsdProcFileOut)

	return parseNfsdStats(data)
}

func getExpectedOSNfsStats() *NfsStats {
	nfsNetStats := &nfsNetStats{
		netCount:           8,
		udpCount:           843,
		tcpCount:           666,
		tcpConnectionCount: 157,
	}

	nfsRPCStats := &nfsRPCStats{
		rpcCount:         220,
		retransmitCount:  789,
		authRefreshCount: 662,
	}

	nfsV3ProcedureStats := []callStats{
		{nfsVersion: 3, nfsCallName: "NULL", nfsCallCount: 191},
		{nfsVersion: 3, nfsCallName: "GETATTR", nfsCallCount: 360},
		{nfsVersion: 3, nfsCallName: "SETATTR", nfsCallCount: 118},
		{nfsVersion: 3, nfsCallName: "LOOKUP", nfsCallCount: 397},
		{nfsVersion: 3, nfsCallName: "ACCESS", nfsCallCount: 455},
		{nfsVersion: 3, nfsCallName: "READLINK", nfsCallCount: 77},
		{nfsVersion: 3, nfsCallName: "READ", nfsCallCount: 581},
		{nfsVersion: 3, nfsCallName: "WRITE", nfsCallCount: 182},
		{nfsVersion: 3, nfsCallName: "CREATE", nfsCallCount: 333},
		{nfsVersion: 3, nfsCallName: "MKDIR", nfsCallCount: 767},
		{nfsVersion: 3, nfsCallName: "SYMLINK", nfsCallCount: 235},
		{nfsVersion: 3, nfsCallName: "MKNOD", nfsCallCount: 558},
		{nfsVersion: 3, nfsCallName: "REMOVE", nfsCallCount: 371},
		{nfsVersion: 3, nfsCallName: "RMDIR", nfsCallCount: 872},
		{nfsVersion: 3, nfsCallName: "RENAME", nfsCallCount: 877},
		{nfsVersion: 3, nfsCallName: "LINK", nfsCallCount: 652},
		{nfsVersion: 3, nfsCallName: "READDIR", nfsCallCount: 265},
		{nfsVersion: 3, nfsCallName: "READDIRPLUS", nfsCallCount: 463},
		{nfsVersion: 3, nfsCallName: "FSSTAT", nfsCallCount: 416},
		{nfsVersion: 3, nfsCallName: "FSINFO", nfsCallCount: 200},
		{nfsVersion: 3, nfsCallName: "PATHCONF", nfsCallCount: 942},
		{nfsVersion: 3, nfsCallName: "COMMIT", nfsCallCount: 235},
	}

	nfsV4OperationStats := []callStats{
		{nfsVersion: 4, nfsCallName: "NULL", nfsCallCount: 32},
		{nfsVersion: 4, nfsCallName: "READ", nfsCallCount: 829},
		{nfsVersion: 4, nfsCallName: "WRITE", nfsCallCount: 218},
		{nfsVersion: 4, nfsCallName: "COMMIT", nfsCallCount: 185},
		{nfsVersion: 4, nfsCallName: "OPEN", nfsCallCount: 789},
		{nfsVersion: 4, nfsCallName: "OPEN_CONFIRM", nfsCallCount: 767},
		{nfsVersion: 4, nfsCallName: "OPEN_NOATTR", nfsCallCount: 813},
		{nfsVersion: 4, nfsCallName: "OPEN_DOWNGRADE", nfsCallCount: 512},
		{nfsVersion: 4, nfsCallName: "CLOSE", nfsCallCount: 615},
		{nfsVersion: 4, nfsCallName: "SETATTR", nfsCallCount: 355},
		{nfsVersion: 4, nfsCallName: "FSINFO", nfsCallCount: 207},
		{nfsVersion: 4, nfsCallName: "RENEW", nfsCallCount: 211},
		{nfsVersion: 4, nfsCallName: "SETCLIENTID", nfsCallCount: 992},
		{nfsVersion: 4, nfsCallName: "SETCLIENTID_CONFIRM", nfsCallCount: 234},
		{nfsVersion: 4, nfsCallName: "LOCK", nfsCallCount: 736},
		{nfsVersion: 4, nfsCallName: "LOCKT", nfsCallCount: 629},
		{nfsVersion: 4, nfsCallName: "LOCKU", nfsCallCount: 862},
		{nfsVersion: 4, nfsCallName: "ACCESS", nfsCallCount: 860},
		{nfsVersion: 4, nfsCallName: "GETATTR", nfsCallCount: 117},
		{nfsVersion: 4, nfsCallName: "LOOKUP", nfsCallCount: 752},
		{nfsVersion: 4, nfsCallName: "LOOKUP_ROOT", nfsCallCount: 128},
		{nfsVersion: 4, nfsCallName: "REMOVE", nfsCallCount: 200},
		{nfsVersion: 4, nfsCallName: "RENAME", nfsCallCount: 494},
		{nfsVersion: 4, nfsCallName: "LINK", nfsCallCount: 372},
		{nfsVersion: 4, nfsCallName: "SYMLINK", nfsCallCount: 158},
		{nfsVersion: 4, nfsCallName: "CREATE", nfsCallCount: 420},
		{nfsVersion: 4, nfsCallName: "PATHCONF", nfsCallCount: 757},
		{nfsVersion: 4, nfsCallName: "STATFS", nfsCallCount: 783},
		{nfsVersion: 4, nfsCallName: "READLINK", nfsCallCount: 46},
		{nfsVersion: 4, nfsCallName: "READDIR", nfsCallCount: 725},
		{nfsVersion: 4, nfsCallName: "SERVER_CAPS", nfsCallCount: 180},
		{nfsVersion: 4, nfsCallName: "DELEGRETURN", nfsCallCount: 301},
		{nfsVersion: 4, nfsCallName: "GETACL", nfsCallCount: 305},
		{nfsVersion: 4, nfsCallName: "SETACL", nfsCallCount: 856},
		{nfsVersion: 4, nfsCallName: "FS_LOCATIONS", nfsCallCount: 965},
		{nfsVersion: 4, nfsCallName: "RELEASE_LOCKOWNER", nfsCallCount: 416},
		{nfsVersion: 4, nfsCallName: "SECINFO", nfsCallCount: 653},
		{nfsVersion: 4, nfsCallName: "FSID_PRESENT", nfsCallCount: 340},
		{nfsVersion: 4, nfsCallName: "EXCHANGE_ID", nfsCallCount: 500},
		{nfsVersion: 4, nfsCallName: "CREATE_SESSION", nfsCallCount: 650},
		{nfsVersion: 4, nfsCallName: "DESTROY_SESSION", nfsCallCount: 545},
		{nfsVersion: 4, nfsCallName: "SEQUENCE", nfsCallCount: 155},
		{nfsVersion: 4, nfsCallName: "GET_LEASE_TIME", nfsCallCount: 620},
		{nfsVersion: 4, nfsCallName: "RECLAIM_COMPLETE", nfsCallCount: 354},
		{nfsVersion: 4, nfsCallName: "GETDEVICEINFO", nfsCallCount: 959},
		{nfsVersion: 4, nfsCallName: "LAYOUTGET", nfsCallCount: 965},
		{nfsVersion: 4, nfsCallName: "LAYOUTCOMMIT", nfsCallCount: 986},
		{nfsVersion: 4, nfsCallName: "LAYOUTRETURN", nfsCallCount: 526},
		{nfsVersion: 4, nfsCallName: "SECINFO_NO_NAME", nfsCallCount: 367},
		{nfsVersion: 4, nfsCallName: "TEST_STATEID", nfsCallCount: 388},
		{nfsVersion: 4, nfsCallName: "FREE_STATEID", nfsCallCount: 373},
		{nfsVersion: 4, nfsCallName: "GETDEVICELIST", nfsCallCount: 786},
		{nfsVersion: 4, nfsCallName: "BIND_CONN_TO_SESSION", nfsCallCount: 890},
		{nfsVersion: 4, nfsCallName: "DESTROY_CLIENTID", nfsCallCount: 459},
		{nfsVersion: 4, nfsCallName: "SEEK", nfsCallCount: 810},
		{nfsVersion: 4, nfsCallName: "ALLOCATE", nfsCallCount: 679},
		{nfsVersion: 4, nfsCallName: "DEALLOCATE", nfsCallCount: 939},
		{nfsVersion: 4, nfsCallName: "LAYOUTSTATS", nfsCallCount: 583},
		{nfsVersion: 4, nfsCallName: "CLONE", nfsCallCount: 790},
		{nfsVersion: 4, nfsCallName: "COPY", nfsCallCount: 333},
		{nfsVersion: 4, nfsCallName: "OFFLOAD_CANCEL", nfsCallCount: 455},
		{nfsVersion: 4, nfsCallName: "COPY_NOTIFY", nfsCallCount: 115},
		{nfsVersion: 4, nfsCallName: "LOOKUPP", nfsCallCount: 155},
		{nfsVersion: 4, nfsCallName: "LAYOUTERROR", nfsCallCount: 884},
		{nfsVersion: 4, nfsCallName: "GETXATTR", nfsCallCount: 571},
		{nfsVersion: 4, nfsCallName: "SETXATTR", nfsCallCount: 409},
		{nfsVersion: 4, nfsCallName: "LISTXATTRS", nfsCallCount: 540},
		{nfsVersion: 4, nfsCallName: "REMOVEXATTR", nfsCallCount: 293},
		{nfsVersion: 4, nfsCallName: "READ_PLUS", nfsCallCount: 721},
	}

	return &NfsStats{
		nfsNetStats:         nfsNetStats,
		nfsRPCStats:         nfsRPCStats,
		nfsV3ProcedureStats: nfsV3ProcedureStats,
		nfsV4OperationStats: nfsV4OperationStats,
	}
}

func getExpectedOSnfsdStats() *nfsdStats {
	repcacheStats := &nfsdRepcacheStats{
		hits:    795,
		misses:  819,
		nocache: 351,
	}

	fhStats := &nfsdFhStats{
		stale: 709,
	}

	ioStats := &nfsdIoStats{
		Read:  111,
		write: 464,
	}

	threadStats := &nfsdThreadStats{
		threads: 261,
	}

	netStats := &nfsdNetStats{
		netCount:           1,
		udpCount:           43,
		tcpCount:           26,
		tcpConnectionCount: 597,
	}

	rpcStats := &nfsdRPCStats{
		rpcCount:       872,
		badCount:       367,
		badFmtCount:    960,
		badAuthCount:   94,
		badClientCount: 748,
	}

	nfsdV3ProcedureStats := []callStats{
		{nfsVersion: 3, nfsCallName: "NULL", nfsCallCount: 124},
		{nfsVersion: 3, nfsCallName: "GETATTR", nfsCallCount: 554},
		{nfsVersion: 3, nfsCallName: "SETATTR", nfsCallCount: 529},
		{nfsVersion: 3, nfsCallName: "LOOKUP", nfsCallCount: 64},
		{nfsVersion: 3, nfsCallName: "ACCESS", nfsCallCount: 928},
		{nfsVersion: 3, nfsCallName: "READLINK", nfsCallCount: 316},
		{nfsVersion: 3, nfsCallName: "READ", nfsCallCount: 531},
		{nfsVersion: 3, nfsCallName: "WRITE", nfsCallCount: 43},
		{nfsVersion: 3, nfsCallName: "CREATE", nfsCallCount: 724},
		{nfsVersion: 3, nfsCallName: "MKDIR", nfsCallCount: 822},
		{nfsVersion: 3, nfsCallName: "SYMLINK", nfsCallCount: 237},
		{nfsVersion: 3, nfsCallName: "MKNOD", nfsCallCount: 665},
		{nfsVersion: 3, nfsCallName: "REMOVE", nfsCallCount: 620},
		{nfsVersion: 3, nfsCallName: "RMDIR", nfsCallCount: 22},
		{nfsVersion: 3, nfsCallName: "RENAME", nfsCallCount: 335},
		{nfsVersion: 3, nfsCallName: "LINK", nfsCallCount: 137},
		{nfsVersion: 3, nfsCallName: "READDIR", nfsCallCount: 236},
		{nfsVersion: 3, nfsCallName: "READDIRPLUS", nfsCallCount: 222},
		{nfsVersion: 3, nfsCallName: "FSSTAT", nfsCallCount: 658},
		{nfsVersion: 3, nfsCallName: "FSINFO", nfsCallCount: 654},
		{nfsVersion: 3, nfsCallName: "PATHCONF", nfsCallCount: 209},
		{nfsVersion: 3, nfsCallName: "COMMIT", nfsCallCount: 382},
	}

	nfsdV4ProcedureStats := []callStats{
		{nfsVersion: 4, nfsCallName: "NULL", nfsCallCount: 512},
		{nfsVersion: 4, nfsCallName: "COMPOUND", nfsCallCount: 878},
	}

	nfsdV4OperationStats := []callStats{
		{nfsVersion: 4, nfsCallName: "UNUSED0", nfsCallCount: 725},
		{nfsVersion: 4, nfsCallName: "UNUSED1", nfsCallCount: 607},
		{nfsVersion: 4, nfsCallName: "UNUSED2", nfsCallCount: 978},
		{nfsVersion: 4, nfsCallName: "ACCESS", nfsCallCount: 86},
		{nfsVersion: 4, nfsCallName: "CLOSE", nfsCallCount: 442},
		{nfsVersion: 4, nfsCallName: "COMMIT", nfsCallCount: 878},
		{nfsVersion: 4, nfsCallName: "CREATE", nfsCallCount: 262},
		{nfsVersion: 4, nfsCallName: "DELEGPURGE", nfsCallCount: 489},
		{nfsVersion: 4, nfsCallName: "DELEGRETURN", nfsCallCount: 962},
		{nfsVersion: 4, nfsCallName: "GETATTR", nfsCallCount: 909},
		{nfsVersion: 4, nfsCallName: "GETFH", nfsCallCount: 563},
		{nfsVersion: 4, nfsCallName: "LINK", nfsCallCount: 468},
		{nfsVersion: 4, nfsCallName: "LOCK", nfsCallCount: 722},
		{nfsVersion: 4, nfsCallName: "LOCKT", nfsCallCount: 104},
		{nfsVersion: 4, nfsCallName: "LOCKU", nfsCallCount: 47},
		{nfsVersion: 4, nfsCallName: "LOOKUP", nfsCallCount: 214},
		{nfsVersion: 4, nfsCallName: "LOOKUPP", nfsCallCount: 305},
		{nfsVersion: 4, nfsCallName: "NVERIFY", nfsCallCount: 564},
		{nfsVersion: 4, nfsCallName: "OPEN", nfsCallCount: 776},
		{nfsVersion: 4, nfsCallName: "OPENATTR", nfsCallCount: 373},
		{nfsVersion: 4, nfsCallName: "OPEN_CONFIRM", nfsCallCount: 444},
		{nfsVersion: 4, nfsCallName: "OPEN_DOWNGRADE", nfsCallCount: 6},
		{nfsVersion: 4, nfsCallName: "PUTFH", nfsCallCount: 265},
		{nfsVersion: 4, nfsCallName: "PUTPUBFH", nfsCallCount: 163},
		{nfsVersion: 4, nfsCallName: "PUTROOTFH", nfsCallCount: 397},
		{nfsVersion: 4, nfsCallName: "READ", nfsCallCount: 817},
		{nfsVersion: 4, nfsCallName: "READDIR", nfsCallCount: 73},
		{nfsVersion: 4, nfsCallName: "READLINK", nfsCallCount: 90},
		{nfsVersion: 4, nfsCallName: "REMOVE", nfsCallCount: 630},
		{nfsVersion: 4, nfsCallName: "RENAME", nfsCallCount: 664},
		{nfsVersion: 4, nfsCallName: "RENEW", nfsCallCount: 984},
		{nfsVersion: 4, nfsCallName: "RESTOREFH", nfsCallCount: 981},
		{nfsVersion: 4, nfsCallName: "SAVEFH", nfsCallCount: 502},
		{nfsVersion: 4, nfsCallName: "SECINFO", nfsCallCount: 682},
		{nfsVersion: 4, nfsCallName: "SETATTR", nfsCallCount: 210},
		{nfsVersion: 4, nfsCallName: "SETCLIENTID", nfsCallCount: 639},
		{nfsVersion: 4, nfsCallName: "SETCLIENTID_CONFIRM", nfsCallCount: 484},
		{nfsVersion: 4, nfsCallName: "VERIFY", nfsCallCount: 924},
		{nfsVersion: 4, nfsCallName: "WRITE", nfsCallCount: 337},
		{nfsVersion: 4, nfsCallName: "RELEASE_LOCKOWNER", nfsCallCount: 857},
		{nfsVersion: 4, nfsCallName: "BACKCHANNEL_CTL", nfsCallCount: 667},
		{nfsVersion: 4, nfsCallName: "BIND_CONN_TO_SESSION", nfsCallCount: 984},
		{nfsVersion: 4, nfsCallName: "EXCHANGE_ID", nfsCallCount: 498},
		{nfsVersion: 4, nfsCallName: "CREATE_SESSION", nfsCallCount: 76},
		{nfsVersion: 4, nfsCallName: "DESTROY_SESSION", nfsCallCount: 515},
		{nfsVersion: 4, nfsCallName: "FREE_STATEID", nfsCallCount: 657},
		{nfsVersion: 4, nfsCallName: "GET_DIR_DELEGATION", nfsCallCount: 596},
		{nfsVersion: 4, nfsCallName: "GETDEVICEINFO", nfsCallCount: 31},
		{nfsVersion: 4, nfsCallName: "GETDEVICELIST", nfsCallCount: 781},
		{nfsVersion: 4, nfsCallName: "LAYOUTCOMMIT", nfsCallCount: 437},
		{nfsVersion: 4, nfsCallName: "LAYOUTGET", nfsCallCount: 23},
		{nfsVersion: 4, nfsCallName: "LAYOUTRETURN", nfsCallCount: 846},
		{nfsVersion: 4, nfsCallName: "SECINFO_NO_NAME", nfsCallCount: 867},
		{nfsVersion: 4, nfsCallName: "SEQUENCE", nfsCallCount: 241},
		{nfsVersion: 4, nfsCallName: "SET_SSV", nfsCallCount: 648},
		{nfsVersion: 4, nfsCallName: "TEST_STATEID", nfsCallCount: 169},
		{nfsVersion: 4, nfsCallName: "WANT_DELEGATION", nfsCallCount: 64},
		{nfsVersion: 4, nfsCallName: "DESTROY_CLIENTID", nfsCallCount: 151},
		{nfsVersion: 4, nfsCallName: "RECLAIM_COMPLETE", nfsCallCount: 447},
		{nfsVersion: 4, nfsCallName: "ALLOCATE", nfsCallCount: 848},
		{nfsVersion: 4, nfsCallName: "COPY", nfsCallCount: 625},
		{nfsVersion: 4, nfsCallName: "COPY_NOTIFY", nfsCallCount: 185},
		{nfsVersion: 4, nfsCallName: "DEALLOCATE", nfsCallCount: 586},
		{nfsVersion: 4, nfsCallName: "IO_ADVISE", nfsCallCount: 890},
		{nfsVersion: 4, nfsCallName: "LAYOUTERROR", nfsCallCount: 446},
		{nfsVersion: 4, nfsCallName: "LAYOUTSTATS", nfsCallCount: 317},
		{nfsVersion: 4, nfsCallName: "OFFLOAD_CANCEL", nfsCallCount: 503},
		{nfsVersion: 4, nfsCallName: "OFFLOAD_STATUS", nfsCallCount: 32},
		{nfsVersion: 4, nfsCallName: "READ_PLUS", nfsCallCount: 935},
		{nfsVersion: 4, nfsCallName: "SEEK", nfsCallCount: 459},
		{nfsVersion: 4, nfsCallName: "WRITE_SAME", nfsCallCount: 386},
		{nfsVersion: 4, nfsCallName: "CLONE", nfsCallCount: 291},
		{nfsVersion: 4, nfsCallName: "GETXATTR", nfsCallCount: 817},
		{nfsVersion: 4, nfsCallName: "SETXATTR", nfsCallCount: 74},
		{nfsVersion: 4, nfsCallName: "LISTXATTRS", nfsCallCount: 592},
		{nfsVersion: 4, nfsCallName: "REMOVEXATTR", nfsCallCount: 562},
	}

	stats := &nfsdStats{
		nfsdRepcacheStats:    repcacheStats,
		nfsdFhStats:          fhStats,
		nfsdIoStats:          ioStats,
		nfsdThreadStats:      threadStats,
		nfsdNetStats:         netStats,
		nfsdRPCStats:         rpcStats,
		nfsdV3ProcedureStats: nfsdV3ProcedureStats,
		nfsdV4ProcedureStats: nfsdV4ProcedureStats,
		nfsdV4OperationStats: nfsdV4OperationStats,
	}

	return stats
}

func TestOSScrape(t *testing.T) {
	if !supportedOS {
		t.Skip()
	}

	type testCase struct {
		name string
	}

	testCases := []testCase{
		{
			name: "All metrics",
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			expectedNfsStats := getExpectedOSNfsStats()
			expectedNfsdStats := getExpectedOSnfsdStats()

			nfsStats, err := mockGetOSNfsStats()
			require.NoError(t, err)

			nfsdStats, err := mockGetOSnfsdStats()
			require.NoError(t, err)

			assert.Equal(t, expectedNfsStats.nfsNetStats.netCount, nfsStats.nfsNetStats.netCount)

			assert.Equal(t, expectedNfsdStats.nfsdNetStats.netCount, nfsdStats.nfsdNetStats.netCount)
		})
	}
}
