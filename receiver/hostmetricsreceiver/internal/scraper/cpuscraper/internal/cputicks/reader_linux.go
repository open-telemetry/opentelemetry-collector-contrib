// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package cputicks // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper/internal/cputicks"

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Reader parses /proc/stat for per-CPU tick counts.
type Reader struct {
	path           string
	ticksPerSecond uint64
}

// NewReader returns a Reader that parses /proc/stat.
// rootPath is the host root directory (e.g. "/host" for container environments);
// pass "" to use the default /proc/stat.
// ticksPerSecond is the OS timer resolution (USER_HZ, typically 100 on Linux).
func NewReader(rootPath string, ticksPerSecond uint64) *Reader {
	if rootPath == "" {
		rootPath = "/"
	}
	return &Reader{path: filepath.Join(rootPath, "proc", "stat"), ticksPerSecond: ticksPerSecond}
}

func (r *Reader) TicksPerSecond() uint64 { return r.ticksPerSecond }

func (r *Reader) ReadAll() ([]Stat, error) {
	f, err := os.Open(r.path)
	if err != nil {
		return nil, fmt.Errorf("cputicks: %w", err)
	}
	defer f.Close()

	result := []Stat{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "cpu") {
			continue
		}
		fields := strings.Fields(line)
		// Skip the aggregate "cpu" line (no digit after "cpu").
		if fields[0] == "cpu" {
			continue
		}
		t, err := parseLine(fields)
		if err != nil {
			return nil, fmt.Errorf("cputicks: %w", err)
		}
		result = append(result, t)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("cputicks: %w", err)
	}
	return result, nil
}

// parseLine converts a split /proc/stat "cpuN" line into a Stat value.
// Minimum 5 fields: cpuN user nice system idle. Extra fields are optional.
func parseLine(fields []string) (Stat, error) {
	if len(fields) < 5 {
		return Stat{}, fmt.Errorf("too few fields in line: %q", strings.Join(fields, " "))
	}
	t := Stat{CPU: fields[0]}

	dests := []*uint64{
		&t.User, &t.Nice, &t.System, &t.Idle, &t.Iowait,
		&t.Irq, &t.Softirq, &t.Steal, &t.Guest, &t.GuestNice,
	}
	for i, dest := range dests {
		col := i + 1
		if col >= len(fields) {
			break
		}
		v, err := strconv.ParseUint(fields[col], 10, 64)
		if err != nil {
			return Stat{}, fmt.Errorf("column %d (%q): %w", col, fields[col], err)
		}
		*dest = v
	}
	return t, nil
}
