// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build darwin

package memoryscraper

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

// parseVMStatCompressorPages is a test helper that exercises the same regex + parsing
// behavior as readVMStatCompressorPages, but without executing vm_stat.
func parseVMStatCompressorPages(out []byte) (int64, error) {
	m := reVMStatCompressor.FindSubmatch(out)
	if len(m) < 2 {
		return 0, errors.New("could not find 'Pages occupied by compressor' in vm_stat output")
	}
	raw := bytes.TrimSpace(m[1])
	v, err := strconv.ParseInt(string(raw), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse int from %q: %w", string(raw), err)
	}
	return v, nil
}

// parsePressureOutput is a test helper that validates the trimming + int parsing
// logic used by readMemorystatusVMPressureLevel (without executing sysctl).
func parsePressureOutput(out []byte) (int64, error) {
	s := string(bytes.TrimSpace(out))
	if s == "" {
		return 0, errors.New("empty sysctl output")
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, err
	}
	return v, nil
}

func TestParseVMStatCompressorPages(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		out     string
		want    int64
		wantErr bool
	}{
		{
			name: "parses_pages",
			out: `
Mach Virtual Memory Statistics: (page size of 16384 bytes)
Pages free:                               123.
Pages active:                             456.
Pages occupied by compressor:             7890.
Pages inactive:                           321.
`,
			want: 7890,
		},
		{
			name: "not_found",
			out: `
Mach Virtual Memory Statistics: (page size of 16384 bytes)
Pages free:                               123.
`,
			wantErr: true,
		},
		{
			name: "bad_number",
			out: `
Pages occupied by compressor:             abc.
`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseVMStatCompressorPages([]byte(tt.out))
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestParsePressureOutput(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		out     []byte
		want    int64
		wantErr bool
	}{
		{
			name: "parses_int_with_newline",
			out:  []byte("2\n"),
			want: 2,
		},
		{
			name: "parses_int_with_spaces",
			out:  []byte("  4  "),
			want: 4,
		},
		{
			name:    "empty",
			out:     []byte("\n"),
			wantErr: true,
		},
		{
			name:    "non_int",
			out:     []byte("warning\n"),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parsePressureOutput(tt.out)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
