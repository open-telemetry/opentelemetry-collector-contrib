package sqlserverreceiver

import "testing"

func TestWaitTypeCode(t *testing.T) {
	tests := []struct {
		code uint
		want string
	}{
		{0, "Unknown"},
		{1, "CPU"},
		{2, "Worker Thread"},
		{3, "Lock"},
		{4, "Latch"},
		{5, "Buffer Latch"},
		{6, "Buffer IO"},
		{7, "Compilation"},
		{8, "SQL CLR"},
		{9, "Mirroring"},
		{10, "Transaction"},
		{11, "Idle"},
		{12, "Preemptive"},
		{13, "Service Broker"},
		{14, "Tran Log IO"},
		{15, "Network IO"},
		{16, "Parallelism"},
		{17, "Memory"},
		{18, "User Wait"},
		{19, "Tracing"},
		{20, "Full Text Search"},
		{21, "Other Disk IO"},
		{22, "Replication"},
		{23, "Log Rate Governor"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := waitTypes[tt.code]
			if got != tt.want {
				t.Errorf("testWaitTypeCode(%v) = (%q), want (%q)", tt.code, got, tt.want)
			}
		})
	}
}
