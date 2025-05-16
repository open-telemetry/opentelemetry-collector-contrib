// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux

package nfsscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/nfsscraper"

func (s *processScraper) recordCPUTimeMetric(now pcommon.Timestamp, cpuTime *cpu.TimesStat) {
}

func (s *processScraper) recordCPUUtilization(now pcommon.Timestamp, cpuUtilization ucal.CPUUtilization) {
}

func getNfsStats() (string, error) {
	return "", nil
}

func getProcessCgroup(ctx context.Context, proc processHandle) (string, error) {
	return "", nil
}

func getProcessExecutable(context.Context, processHandle) (string, error) {
	return "", nil
}

func getProcessCommand(context.Context, processHandle) (*commandMetadata, error) {
	return nil, nil
}
