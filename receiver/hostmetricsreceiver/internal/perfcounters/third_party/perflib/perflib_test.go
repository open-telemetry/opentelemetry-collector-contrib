//go:build windows

package perflib

import (
	"testing"
)

func BenchmarkQueryPerformanceData(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = QueryPerformanceData("Global", "")
	}
}
