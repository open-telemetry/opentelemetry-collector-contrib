// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package cputicks

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadAll(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []Stat
		wantErr string
	}{
		{
			name: "standard multi-cpu",
			content: `cpu  100 200 300 400 50 60 70 80 90 10
cpu0 10 20 30 40 5 6 7 8 9 1
cpu1 90 180 270 360 45 54 63 72 81 9
`,
			want: []Stat{
				{CPU: "cpu0", User: 10, Nice: 20, System: 30, Idle: 40, Iowait: 5, Irq: 6, Softirq: 7, Steal: 8, Guest: 9, GuestNice: 1},
				{CPU: "cpu1", User: 90, Nice: 180, System: 270, Idle: 360, Iowait: 45, Irq: 54, Softirq: 63, Steal: 72, Guest: 81, GuestNice: 9},
			},
		},
		{
			name: "minimal four columns (old kernel)",
			content: `cpu  100 200 300 400
cpu0 10 20 30 40
`,
			want: []Stat{
				{CPU: "cpu0", User: 10, Nice: 20, System: 30, Idle: 40},
			},
		},
		{
			name: "full ten columns",
			content: `cpu  1 2 3 4 5 6 7 8 9 10
cpu0 1 2 3 4 5 6 7 8 9 10
`,
			want: []Stat{
				{CPU: "cpu0", User: 1, Nice: 2, System: 3, Idle: 4, Iowait: 5, Irq: 6, Softirq: 7, Steal: 8, Guest: 9, GuestNice: 10},
			},
		},
		{
			name:    "aggregate cpu line skipped",
			content: "cpu  100 200 300 400 50 60 70 80 90 10\nintr 123 456\n",
			want:    []Stat{},
		},
		{
			name:    "malformed non-numeric field",
			content: "cpu  100 200 300 400\ncpu0 10 abc 30 40\n",
			wantErr: `column 2 ("abc")`,
		},
		{
			name:    "empty file",
			content: "",
			want:    []Stat{},
		},
		{
			name: "non-cpu lines interspersed",
			content: `cpu  100 200 300 400 50 60 70 80 90 10
intr 1234 56 78
cpu0 10 20 30 40 5 6 7 8 9 1
ctxt 9876543
cpu1 90 180 270 360 45 54 63 72 81 9
processes 12345
`,
			want: []Stat{
				{CPU: "cpu0", User: 10, Nice: 20, System: 30, Idle: 40, Iowait: 5, Irq: 6, Softirq: 7, Steal: 8, Guest: 9, GuestNice: 1},
				{CPU: "cpu1", User: 90, Nice: 180, System: 270, Idle: 360, Iowait: 45, Irq: 54, Softirq: 63, Steal: 72, Guest: 81, GuestNice: 9},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "stat")
			require.NoError(t, os.WriteFile(path, []byte(tt.content), 0o600))

			r := &Reader{path: path, ticksPerSecond: 100}
			actual, err := r.ReadAll()
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, actual)
		})
	}
}

func TestReadAll_FileNotFound(t *testing.T) {
	r := &Reader{path: "/nonexistent/stat", ticksPerSecond: 100}
	_, err := r.ReadAll()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cputicks:")
}

func TestNewReader_RootPath(t *testing.T) {
	root := t.TempDir()
	procDir := filepath.Join(root, "proc")
	require.NoError(t, os.MkdirAll(procDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(procDir, "stat"), []byte(
		"cpu  100 200 300 400 50 60 70 80 90 10\ncpu0 10 20 30 40 5 6 7 8 9 1\n",
	), 0o600))

	r := NewReader(root, 100)
	assert.Equal(t, uint64(100), r.TicksPerSecond())
	ticks, err := r.ReadAll()
	require.NoError(t, err)
	require.Len(t, ticks, 1)
	assert.Equal(t, Stat{CPU: "cpu0", User: 10, Nice: 20, System: 30, Idle: 40, Iowait: 5, Irq: 6, Softirq: 7, Steal: 8, Guest: 9, GuestNice: 1}, ticks[0])
}

func TestReadAll_RealProcStat(t *testing.T) {
	if _, err := os.Stat("/proc/stat"); err != nil {
		t.Skip("/proc/stat not available")
	}
	r := NewReader("", 100)
	ticks, err := r.ReadAll()
	require.NoError(t, err)
	require.NotEmpty(t, ticks, "expected at least one CPU")
	for _, tick := range ticks {
		assert.NotEmpty(t, tick.CPU)
		assert.NotZero(t, tick.Total(), "expected non-zero total for %s", tick.CPU)
	}
}

func BenchmarkReadAll(b *testing.B) {
	if _, err := os.Stat("/proc/stat"); err != nil {
		b.Skip("/proc/stat not available")
	}
	r := NewReader("", 100)

	b.ResetTimer()
	for range b.N {
		_, err := r.ReadAll()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkReadAll_Gopsutil(b *testing.B) {
	if _, err := os.Stat("/proc/stat"); err != nil {
		b.Skip("/proc/stat not available")
	}

	b.ResetTimer()
	for range b.N {
		_, err := cpu.TimesWithContext(b.Context(), true)
		if err != nil {
			b.Fatal(err)
		}
	}
}
