package batchmemlimitprocessor

import (
	"fmt"
	"go.opentelemetry.io/collector/pdata/plog"
	"testing"
)

func TestBatchMemoryLimitProcessor_extractBatch(t *testing.T) {
	processor := batchMemoryLimitProcessor{
		config: &Config{MemoryLimit: 100},
	}

	tests := []struct {
		name string
		ld   plog.Logs
	}{
		{
			name: "one resource",
			ld:   generateTestLogOneResourceOneScope(),
		},
		{
			name: "one resource few scopes",
			ld:   generateTestLogOneResourceFewScopes(),
		},
		{
			name: "few resources few scopes",
			ld:   generateTestLogFewResourceFewScopes(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for {
				res := processor.extractBatch(tt.ld)
				if res.LogRecordCount() == 0 {
					return
				}
			}
		})
	}
}

func generateTestLogOneResourceOneScope() plog.Logs {
	ld := plog.NewLogs()

	rl0 := ld.ResourceLogs().AppendEmpty()
	sc := rl0.ScopeLogs().AppendEmpty()
	for i := 0; i < 100; i++ {
		el := sc.LogRecords().AppendEmpty()
		el.Body().SetStringVal(fmt.Sprintf("This is entry # %q", i))
	}

	return ld
}

func generateTestLogOneResourceFewScopes() plog.Logs {
	ld := plog.NewLogs()

	rl0 := ld.ResourceLogs().AppendEmpty()
	for k := 0; k < 3; k++ {
		sc := rl0.ScopeLogs().AppendEmpty()
		sc.Scope().SetName(fmt.Sprintf("Scope %q", k))
		for i := 0; i < 100; i++ {
			el := sc.LogRecords().AppendEmpty()
			el.Body().SetStringVal(fmt.Sprintf("This is entry # %q", i))
		}
	}

	return ld
}

func generateTestLogFewResourceFewScopes() plog.Logs {
	ld := plog.NewLogs()

	for z := 0; z < 3; z++ {
		rl := ld.ResourceLogs().AppendEmpty()
		for k := 0; k < 3; k++ {
			sc := rl.ScopeLogs().AppendEmpty()
			sc.Scope().SetName(fmt.Sprintf("Scope %q", k))
			for i := 0; i < 100; i++ {
				el := sc.LogRecords().AppendEmpty()
				el.Body().SetStringVal(fmt.Sprintf("This is entry # %q", i))
			}
		}
	}

	return ld
}
