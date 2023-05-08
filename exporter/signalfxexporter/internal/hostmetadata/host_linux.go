// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build linux
// +build linux

// Taken from https://github.com/signalfx/golib/blob/master/metadata/hostmetadata/host-linux.go
// with minor modifications.

package hostmetadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/hostmetadata"

import (
	"bytes"

	"golang.org/x/sys/unix"
)

// syscallUname maps to the golib system call, but can be modified for testing
var syscallUname = unix.Uname

func fillPlatformSpecificOSData(info *hostOS) error {
	info.HostLinuxVersion, _ = getLinuxVersion()

	uname := &unix.Utsname{}
	if err := syscallUname(uname); err != nil {
		return err
	}

	info.HostKernelVersion = string(bytes.Trim(uname.Version[:], "\x00"))
	return nil
}

func fillPlatformSpecificCPUData(info *hostCPU) error {
	uname := &unix.Utsname{}
	if err := syscallUname(uname); err != nil {
		return err
	}

	info.HostMachine = string(bytes.Trim(uname.Machine[:], "\x00"))

	// according to the python doc platform.Processor usually returns the same
	// value as platform.Machine
	// https://docs.python.org/3/library/platform.html#platform.processor
	info.HostProcessor = info.HostMachine
	return nil
}
