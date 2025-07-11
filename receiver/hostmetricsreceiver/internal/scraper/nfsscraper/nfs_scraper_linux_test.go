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
		NetCount:           8,
		UDPCount:           843,
		TCPCount:           666,
		TCPConnectionCount: 157,
	}

	nfsRPCStats := &nfsRPCStats{
		RPCCount:         220,
		RetransmitCount:  789,
		AuthRefreshCount: 662,
	}

	nfsV3ProcedureStats := []callStats{
		{NFSVersion: 3, NFSCallName: "NULL", NFSCallCount: 191},
		{NFSVersion: 3, NFSCallName: "GETATTR", NFSCallCount: 360},
		{NFSVersion: 3, NFSCallName: "SETATTR", NFSCallCount: 118},
		{NFSVersion: 3, NFSCallName: "LOOKUP", NFSCallCount: 397},
		{NFSVersion: 3, NFSCallName: "ACCESS", NFSCallCount: 455},
		{NFSVersion: 3, NFSCallName: "READLINK", NFSCallCount: 77},
		{NFSVersion: 3, NFSCallName: "READ", NFSCallCount: 581},
		{NFSVersion: 3, NFSCallName: "WRITE", NFSCallCount: 182},
		{NFSVersion: 3, NFSCallName: "CREATE", NFSCallCount: 333},
		{NFSVersion: 3, NFSCallName: "MKDIR", NFSCallCount: 767},
		{NFSVersion: 3, NFSCallName: "SYMLINK", NFSCallCount: 235},
		{NFSVersion: 3, NFSCallName: "MKNOD", NFSCallCount: 558},
		{NFSVersion: 3, NFSCallName: "REMOVE", NFSCallCount: 371},
		{NFSVersion: 3, NFSCallName: "RMDIR", NFSCallCount: 872},
		{NFSVersion: 3, NFSCallName: "RENAME", NFSCallCount: 877},
		{NFSVersion: 3, NFSCallName: "LINK", NFSCallCount: 652},
		{NFSVersion: 3, NFSCallName: "READDIR", NFSCallCount: 265},
		{NFSVersion: 3, NFSCallName: "READDIRPLUS", NFSCallCount: 463},
		{NFSVersion: 3, NFSCallName: "FSSTAT", NFSCallCount: 416},
		{NFSVersion: 3, NFSCallName: "FSINFO", NFSCallCount: 200},
		{NFSVersion: 3, NFSCallName: "PATHCONF", NFSCallCount: 942},
		{NFSVersion: 3, NFSCallName: "COMMIT", NFSCallCount: 235},
	}

	nfsV4OperationStats := []callStats{
		{NFSVersion: 4, NFSCallName: "NULL", NFSCallCount: 32},
		{NFSVersion: 4, NFSCallName: "READ", NFSCallCount: 829},
		{NFSVersion: 4, NFSCallName: "WRITE", NFSCallCount: 218},
		{NFSVersion: 4, NFSCallName: "COMMIT", NFSCallCount: 185},
		{NFSVersion: 4, NFSCallName: "OPEN", NFSCallCount: 789},
		{NFSVersion: 4, NFSCallName: "OPEN_CONFIRM", NFSCallCount: 767},
		{NFSVersion: 4, NFSCallName: "OPEN_NOATTR", NFSCallCount: 813},
		{NFSVersion: 4, NFSCallName: "OPEN_DOWNGRADE", NFSCallCount: 512},
		{NFSVersion: 4, NFSCallName: "CLOSE", NFSCallCount: 615},
		{NFSVersion: 4, NFSCallName: "SETATTR", NFSCallCount: 355},
		{NFSVersion: 4, NFSCallName: "FSINFO", NFSCallCount: 207},
		{NFSVersion: 4, NFSCallName: "RENEW", NFSCallCount: 211},
		{NFSVersion: 4, NFSCallName: "SETCLIENTID", NFSCallCount: 992},
		{NFSVersion: 4, NFSCallName: "SETCLIENTID_CONFIRM", NFSCallCount: 234},
		{NFSVersion: 4, NFSCallName: "LOCK", NFSCallCount: 736},
		{NFSVersion: 4, NFSCallName: "LOCKT", NFSCallCount: 629},
		{NFSVersion: 4, NFSCallName: "LOCKU", NFSCallCount: 862},
		{NFSVersion: 4, NFSCallName: "ACCESS", NFSCallCount: 860},
		{NFSVersion: 4, NFSCallName: "GETATTR", NFSCallCount: 117},
		{NFSVersion: 4, NFSCallName: "LOOKUP", NFSCallCount: 752},
		{NFSVersion: 4, NFSCallName: "LOOKUP_ROOT", NFSCallCount: 128},
		{NFSVersion: 4, NFSCallName: "REMOVE", NFSCallCount: 200},
		{NFSVersion: 4, NFSCallName: "RENAME", NFSCallCount: 494},
		{NFSVersion: 4, NFSCallName: "LINK", NFSCallCount: 372},
		{NFSVersion: 4, NFSCallName: "SYMLINK", NFSCallCount: 158},
		{NFSVersion: 4, NFSCallName: "CREATE", NFSCallCount: 420},
		{NFSVersion: 4, NFSCallName: "PATHCONF", NFSCallCount: 757},
		{NFSVersion: 4, NFSCallName: "STATFS", NFSCallCount: 783},
		{NFSVersion: 4, NFSCallName: "READLINK", NFSCallCount: 46},
		{NFSVersion: 4, NFSCallName: "READDIR", NFSCallCount: 725},
		{NFSVersion: 4, NFSCallName: "SERVER_CAPS", NFSCallCount: 180},
		{NFSVersion: 4, NFSCallName: "DELEGRETURN", NFSCallCount: 301},
		{NFSVersion: 4, NFSCallName: "GETACL", NFSCallCount: 305},
		{NFSVersion: 4, NFSCallName: "SETACL", NFSCallCount: 856},
		{NFSVersion: 4, NFSCallName: "FS_LOCATIONS", NFSCallCount: 965},
		{NFSVersion: 4, NFSCallName: "RELEASE_LOCKOWNER", NFSCallCount: 416},
		{NFSVersion: 4, NFSCallName: "SECINFO", NFSCallCount: 653},
		{NFSVersion: 4, NFSCallName: "FSID_PRESENT", NFSCallCount: 340},
		{NFSVersion: 4, NFSCallName: "EXCHANGE_ID", NFSCallCount: 500},
		{NFSVersion: 4, NFSCallName: "CREATE_SESSION", NFSCallCount: 650},
		{NFSVersion: 4, NFSCallName: "DESTROY_SESSION", NFSCallCount: 545},
		{NFSVersion: 4, NFSCallName: "SEQUENCE", NFSCallCount: 155},
		{NFSVersion: 4, NFSCallName: "GET_LEASE_TIME", NFSCallCount: 620},
		{NFSVersion: 4, NFSCallName: "RECLAIM_COMPLETE", NFSCallCount: 354},
		{NFSVersion: 4, NFSCallName: "GETDEVICEINFO", NFSCallCount: 959},
		{NFSVersion: 4, NFSCallName: "LAYOUTGET", NFSCallCount: 965},
		{NFSVersion: 4, NFSCallName: "LAYOUTCOMMIT", NFSCallCount: 986},
		{NFSVersion: 4, NFSCallName: "LAYOUTRETURN", NFSCallCount: 526},
		{NFSVersion: 4, NFSCallName: "SECINFO_NO_NAME", NFSCallCount: 367},
		{NFSVersion: 4, NFSCallName: "TEST_STATEID", NFSCallCount: 388},
		{NFSVersion: 4, NFSCallName: "FREE_STATEID", NFSCallCount: 373},
		{NFSVersion: 4, NFSCallName: "GETDEVICELIST", NFSCallCount: 786},
		{NFSVersion: 4, NFSCallName: "BIND_CONN_TO_SESSION", NFSCallCount: 890},
		{NFSVersion: 4, NFSCallName: "DESTROY_CLIENTID", NFSCallCount: 459},
		{NFSVersion: 4, NFSCallName: "SEEK", NFSCallCount: 810},
		{NFSVersion: 4, NFSCallName: "ALLOCATE", NFSCallCount: 679},
		{NFSVersion: 4, NFSCallName: "DEALLOCATE", NFSCallCount: 939},
		{NFSVersion: 4, NFSCallName: "LAYOUTSTATS", NFSCallCount: 583},
		{NFSVersion: 4, NFSCallName: "CLONE", NFSCallCount: 790},
		{NFSVersion: 4, NFSCallName: "COPY", NFSCallCount: 333},
		{NFSVersion: 4, NFSCallName: "OFFLOAD_CANCEL", NFSCallCount: 455},
		{NFSVersion: 4, NFSCallName: "COPY_NOTIFY", NFSCallCount: 115},
		{NFSVersion: 4, NFSCallName: "LOOKUPP", NFSCallCount: 155},
		{NFSVersion: 4, NFSCallName: "LAYOUTERROR", NFSCallCount: 884},
		{NFSVersion: 4, NFSCallName: "GETXATTR", NFSCallCount: 571},
		{NFSVersion: 4, NFSCallName: "SETXATTR", NFSCallCount: 409},
		{NFSVersion: 4, NFSCallName: "LISTXATTRS", NFSCallCount: 540},
		{NFSVersion: 4, NFSCallName: "REMOVEXATTR", NFSCallCount: 293},
		{NFSVersion: 4, NFSCallName: "READ_PLUS", NFSCallCount: 721},
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
		Hits:    795,
		Misses:  819,
		Nocache: 351,
	}

	fhStats := &nfsdFhStats{
		Stale: 709,
	}

	ioStats := &nfsdIoStats{
		Read:  111,
		Write: 464,
	}

	threadStats := &nfsdThreadStats{
		Threads: 261,
	}

	netStats := &NfsdNetStats{
		NetCount:           1,
		UDPCount:           43,
		TCPCount:           26,
		TCPConnectionCount: 597,
	}

	rpcStats := &NfsdRPCStats{
		RPCCount:       872,
		BadCount:       367,
		BadFmtCount:    960,
		BadAuthCount:   94,
		BadClientCount: 748,
	}

	nfsdV3ProcedureStats := []callStats{
		{NFSVersion: 3, NFSCallName: "NULL", NFSCallCount: 124},
		{NFSVersion: 3, NFSCallName: "GETATTR", NFSCallCount: 554},
		{NFSVersion: 3, NFSCallName: "SETATTR", NFSCallCount: 529},
		{NFSVersion: 3, NFSCallName: "LOOKUP", NFSCallCount: 64},
		{NFSVersion: 3, NFSCallName: "ACCESS", NFSCallCount: 928},
		{NFSVersion: 3, NFSCallName: "READLINK", NFSCallCount: 316},
		{NFSVersion: 3, NFSCallName: "READ", NFSCallCount: 531},
		{NFSVersion: 3, NFSCallName: "WRITE", NFSCallCount: 43},
		{NFSVersion: 3, NFSCallName: "CREATE", NFSCallCount: 724},
		{NFSVersion: 3, NFSCallName: "MKDIR", NFSCallCount: 822},
		{NFSVersion: 3, NFSCallName: "SYMLINK", NFSCallCount: 237},
		{NFSVersion: 3, NFSCallName: "MKNOD", NFSCallCount: 665},
		{NFSVersion: 3, NFSCallName: "REMOVE", NFSCallCount: 620},
		{NFSVersion: 3, NFSCallName: "RMDIR", NFSCallCount: 22},
		{NFSVersion: 3, NFSCallName: "RENAME", NFSCallCount: 335},
		{NFSVersion: 3, NFSCallName: "LINK", NFSCallCount: 137},
		{NFSVersion: 3, NFSCallName: "READDIR", NFSCallCount: 236},
		{NFSVersion: 3, NFSCallName: "READDIRPLUS", NFSCallCount: 222},
		{NFSVersion: 3, NFSCallName: "FSSTAT", NFSCallCount: 658},
		{NFSVersion: 3, NFSCallName: "FSINFO", NFSCallCount: 654},
		{NFSVersion: 3, NFSCallName: "PATHCONF", NFSCallCount: 209},
		{NFSVersion: 3, NFSCallName: "COMMIT", NFSCallCount: 382},
	}

	nfsdV4ProcedureStats := []callStats{
		{NFSVersion: 4, NFSCallName: "NULL", NFSCallCount: 512},
		{NFSVersion: 4, NFSCallName: "COMPOUND", NFSCallCount: 878},
	}

	nfsdV4OperationStats := []callStats{
		{NFSVersion: 4, NFSCallName: "UNUSED0", NFSCallCount: 725},
		{NFSVersion: 4, NFSCallName: "UNUSED1", NFSCallCount: 607},
		{NFSVersion: 4, NFSCallName: "UNUSED2", NFSCallCount: 978},
		{NFSVersion: 4, NFSCallName: "ACCESS", NFSCallCount: 86},
		{NFSVersion: 4, NFSCallName: "CLOSE", NFSCallCount: 442},
		{NFSVersion: 4, NFSCallName: "COMMIT", NFSCallCount: 878},
		{NFSVersion: 4, NFSCallName: "CREATE", NFSCallCount: 262},
		{NFSVersion: 4, NFSCallName: "DELEGPURGE", NFSCallCount: 489},
		{NFSVersion: 4, NFSCallName: "DELEGRETURN", NFSCallCount: 962},
		{NFSVersion: 4, NFSCallName: "GETATTR", NFSCallCount: 909},
		{NFSVersion: 4, NFSCallName: "GETFH", NFSCallCount: 563},
		{NFSVersion: 4, NFSCallName: "LINK", NFSCallCount: 468},
		{NFSVersion: 4, NFSCallName: "LOCK", NFSCallCount: 722},
		{NFSVersion: 4, NFSCallName: "LOCKT", NFSCallCount: 104},
		{NFSVersion: 4, NFSCallName: "LOCKU", NFSCallCount: 47},
		{NFSVersion: 4, NFSCallName: "LOOKUP", NFSCallCount: 214},
		{NFSVersion: 4, NFSCallName: "LOOKUPP", NFSCallCount: 305},
		{NFSVersion: 4, NFSCallName: "NVERIFY", NFSCallCount: 564},
		{NFSVersion: 4, NFSCallName: "OPEN", NFSCallCount: 776},
		{NFSVersion: 4, NFSCallName: "OPENATTR", NFSCallCount: 373},
		{NFSVersion: 4, NFSCallName: "OPEN_CONFIRM", NFSCallCount: 444},
		{NFSVersion: 4, NFSCallName: "OPEN_DOWNGRADE", NFSCallCount: 6},
		{NFSVersion: 4, NFSCallName: "PUTFH", NFSCallCount: 265},
		{NFSVersion: 4, NFSCallName: "PUTPUBFH", NFSCallCount: 163},
		{NFSVersion: 4, NFSCallName: "PUTROOTFH", NFSCallCount: 397},
		{NFSVersion: 4, NFSCallName: "READ", NFSCallCount: 817},
		{NFSVersion: 4, NFSCallName: "READDIR", NFSCallCount: 73},
		{NFSVersion: 4, NFSCallName: "READLINK", NFSCallCount: 90},
		{NFSVersion: 4, NFSCallName: "REMOVE", NFSCallCount: 630},
		{NFSVersion: 4, NFSCallName: "RENAME", NFSCallCount: 664},
		{NFSVersion: 4, NFSCallName: "RENEW", NFSCallCount: 984},
		{NFSVersion: 4, NFSCallName: "RESTOREFH", NFSCallCount: 981},
		{NFSVersion: 4, NFSCallName: "SAVEFH", NFSCallCount: 502},
		{NFSVersion: 4, NFSCallName: "SECINFO", NFSCallCount: 682},
		{NFSVersion: 4, NFSCallName: "SETATTR", NFSCallCount: 210},
		{NFSVersion: 4, NFSCallName: "SETCLIENTID", NFSCallCount: 639},
		{NFSVersion: 4, NFSCallName: "SETCLIENTID_CONFIRM", NFSCallCount: 484},
		{NFSVersion: 4, NFSCallName: "VERIFY", NFSCallCount: 924},
		{NFSVersion: 4, NFSCallName: "WRITE", NFSCallCount: 337},
		{NFSVersion: 4, NFSCallName: "RELEASE_LOCKOWNER", NFSCallCount: 857},
		{NFSVersion: 4, NFSCallName: "BACKCHANNEL_CTL", NFSCallCount: 667},
		{NFSVersion: 4, NFSCallName: "BIND_CONN_TO_SESSION", NFSCallCount: 984},
		{NFSVersion: 4, NFSCallName: "EXCHANGE_ID", NFSCallCount: 498},
		{NFSVersion: 4, NFSCallName: "CREATE_SESSION", NFSCallCount: 76},
		{NFSVersion: 4, NFSCallName: "DESTROY_SESSION", NFSCallCount: 515},
		{NFSVersion: 4, NFSCallName: "FREE_STATEID", NFSCallCount: 657},
		{NFSVersion: 4, NFSCallName: "GET_DIR_DELEGATION", NFSCallCount: 596},
		{NFSVersion: 4, NFSCallName: "GETDEVICEINFO", NFSCallCount: 31},
		{NFSVersion: 4, NFSCallName: "GETDEVICELIST", NFSCallCount: 781},
		{NFSVersion: 4, NFSCallName: "LAYOUTCOMMIT", NFSCallCount: 437},
		{NFSVersion: 4, NFSCallName: "LAYOUTGET", NFSCallCount: 23},
		{NFSVersion: 4, NFSCallName: "LAYOUTRETURN", NFSCallCount: 846},
		{NFSVersion: 4, NFSCallName: "SECINFO_NO_NAME", NFSCallCount: 867},
		{NFSVersion: 4, NFSCallName: "SEQUENCE", NFSCallCount: 241},
		{NFSVersion: 4, NFSCallName: "SET_SSV", NFSCallCount: 648},
		{NFSVersion: 4, NFSCallName: "TEST_STATEID", NFSCallCount: 169},
		{NFSVersion: 4, NFSCallName: "WANT_DELEGATION", NFSCallCount: 64},
		{NFSVersion: 4, NFSCallName: "DESTROY_CLIENTID", NFSCallCount: 151},
		{NFSVersion: 4, NFSCallName: "RECLAIM_COMPLETE", NFSCallCount: 447},
		{NFSVersion: 4, NFSCallName: "ALLOCATE", NFSCallCount: 848},
		{NFSVersion: 4, NFSCallName: "COPY", NFSCallCount: 625},
		{NFSVersion: 4, NFSCallName: "COPY_NOTIFY", NFSCallCount: 185},
		{NFSVersion: 4, NFSCallName: "DEALLOCATE", NFSCallCount: 586},
		{NFSVersion: 4, NFSCallName: "IO_ADVISE", NFSCallCount: 890},
		{NFSVersion: 4, NFSCallName: "LAYOUTERROR", NFSCallCount: 446},
		{NFSVersion: 4, NFSCallName: "LAYOUTSTATS", NFSCallCount: 317},
		{NFSVersion: 4, NFSCallName: "OFFLOAD_CANCEL", NFSCallCount: 503},
		{NFSVersion: 4, NFSCallName: "OFFLOAD_STATUS", NFSCallCount: 32},
		{NFSVersion: 4, NFSCallName: "READ_PLUS", NFSCallCount: 935},
		{NFSVersion: 4, NFSCallName: "SEEK", NFSCallCount: 459},
		{NFSVersion: 4, NFSCallName: "WRITE_SAME", NFSCallCount: 386},
		{NFSVersion: 4, NFSCallName: "CLONE", NFSCallCount: 291},
		{NFSVersion: 4, NFSCallName: "GETXATTR", NFSCallCount: 817},
		{NFSVersion: 4, NFSCallName: "SETXATTR", NFSCallCount: 74},
		{NFSVersion: 4, NFSCallName: "LISTXATTRS", NFSCallCount: 592},
		{NFSVersion: 4, NFSCallName: "REMOVEXATTR", NFSCallCount: 562},
	}

	stats := &nfsdStats{
		nfsdRepcacheStats:    repcacheStats,
		nfsdFhStats:          fhStats,
		nfsdIoStats:          ioStats,
		nfsdThreadStats:      threadStats,
		NfsdNetStats:         netStats,
		NfsdRPCStats:         rpcStats,
		NfsdV3ProcedureStats: nfsdV3ProcedureStats,
		NfsdV4ProcedureStats: nfsdV4ProcedureStats,
		NfsdV4OperationStats: nfsdV4OperationStats,
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

			assert.Equal(t, expectedNfsStats.nfsNetStats.NetCount, nfsStats.nfsNetStats.NetCount)

			assert.Equal(t, expectedNfsdStats.NfsdNetStats.NetCount, nfsdStats.NfsdNetStats.NetCount)
		})
	}
}
