// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package procx // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/sumologicextension/internal/procx"

import "github.com/shirou/gopsutil/v4/process"

type SumoTag string

func (s SumoTag) String() string {
	return string(s)
}

type ProcessIdentifier string

func (p ProcessIdentifier) String() string {
	return string(p)
}

type processWrapper struct {
	process *process.Process
}

func (pw *processWrapper) Pid() int32 {
	return pw.process.Pid
}

func (pw *processWrapper) Name() (string, error) {
	return pw.process.Name()
}

func (pw *processWrapper) Cmdline() (string, error) {
	return pw.process.Cmdline()
}

type Process interface {
	Pid() int32
	Name() (string, error)
	Cmdline() (string, error)
}
