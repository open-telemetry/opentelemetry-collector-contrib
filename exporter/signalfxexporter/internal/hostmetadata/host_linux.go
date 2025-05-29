// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

// Taken from https://github.com/signalfx/golib/blob/master/metadata/hostmetadata/host-linux.go
// with minor modifications.

package hostmetadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/hostmetadata"

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"regexp"

	"golang.org/x/net/context"
	"golang.org/x/sys/unix"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/gopsutilenv"
)

// syscallUname maps to the golib system call, but can be modified for testing
var syscallUname = unix.Uname

func fillPlatformSpecificOSData(ctx context.Context, info *hostOS) error {
	info.HostLinuxVersion, _ = getLinuxVersion(ctx)

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

// getLinuxVersion - adds information about the host linux version to the supplied map
func getLinuxVersion(ctx context.Context) (string, error) {
	etc := gopsutilenv.GetEnvWithContext(ctx, "HOST_ETC", "/etc")
	if value, err := getStringFromFile(`DISTRIB_DESCRIPTION="(.*)"`, filepath.Join(etc, "lsb-release")); err == nil {
		return value, nil
	}
	if value, err := getStringFromFile(`PRETTY_NAME="(.*)"`, filepath.Join(etc, "os-release")); err == nil {
		return value, nil
	}
	if value, err := os.ReadFile(filepath.Join(etc, "centos-release")); err == nil {
		return string(value), nil
	}
	if value, err := os.ReadFile(filepath.Join(etc, "redhat-release")); err == nil {
		return string(value), nil
	}
	if value, err := os.ReadFile(filepath.Join(etc, "system-release")); err == nil {
		return string(value), nil
	}
	return "", errors.New("unable to find linux version")
}

func getStringFromFile(pattern string, path string) (string, error) {
	var err error
	var file []byte
	reg := regexp.MustCompile(pattern)
	if file, err = os.ReadFile(path); err == nil {
		if match := reg.FindSubmatch(file); len(match) > 1 {
			return string(match[1]), nil
		}
	}
	return "", err
}
