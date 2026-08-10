// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package container

import (
	"regexp"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

const (
	benchLogPath       = "/var/log/pods/default_mypod_49cc7c1fd3702c40b2686ea7486091d3/mycontainer/0.log"
	benchContainerdLog = "2024-01-15T10:30:00.000Z stdout F this is a test log line with realistic content"
	benchCRIOLog       = "2024-01-15T10:30:00.000000000+00:00 stdout F this is a test log line with realistic content"
)

// old regex patterns kept here only for benchmarking comparison
var (
	benchCRIOMatcher       = regexp.MustCompile(`^(?P<time>[^ Z]+) (?P<stream>stdout|stderr) (?P<logtag>[^ ]*) ?(?P<log>.*)$`)
	benchContainerdMatcher = regexp.MustCompile(`^(?P<time>[^ ^Z]+Z) (?P<stream>stdout|stderr) (?P<logtag>[^ ]*) ?(?P<log>.*)$`)
	benchPathMatcher       = regexp.MustCompile(`^.*(\/|\\)(?P<namespace>[^_]+)_(?P<pod_name>[^_]+)_(?P<uid>[a-f0-9\-]+)(\/|\\)(?P<container_name>[^\._]+)(\/|\\)(?P<restart_count>\d+)\.log(\.\d{8}-\d{6})?$`)
)

// BenchmarkCRIParsing compares the hand-written CRI scanner against the original
// regex-based approach for both containerd and crio log line formats.
func BenchmarkCRIParsing(b *testing.B) {
	benchmarks := []struct {
		name  string
		input string
		re    *regexp.Regexp
	}{
		{"Containerd", benchContainerdLog, benchContainerdMatcher},
		{"CRIO", benchCRIOLog, benchCRIOMatcher},
	}

	for _, bm := range benchmarks {
		b.Run("Regex/"+bm.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _ = helper.MatchValues(bm.input, bm.re)
			}
		})

		b.Run("NoRegex/"+bm.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _, _ = parseCRI(bm.input)
			}
		})
	}
}

// BenchmarkLogPathParsing compares the hand-written log path parser against the
// original regex-based approach.
func BenchmarkLogPathParsing(b *testing.B) {
	benchmarks := []struct {
		name  string
		input string
	}{
		{"Standard", benchLogPath},
		{"Rotated", benchLogPath + ".20240115-103000"},
	}

	for _, bm := range benchmarks {
		b.Run("Regex/"+bm.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _ = helper.MatchValues(bm.input, benchPathMatcher)
			}
		})

		b.Run("NoRegex/"+bm.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _ = parseLogPath(bm.input)
			}
		})
	}
}
