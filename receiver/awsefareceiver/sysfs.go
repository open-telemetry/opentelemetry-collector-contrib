// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows
// +build !windows

package awsefareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsefareceiver"

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"go.uber.org/zap"
)

const (
	defaultEfaPath = "/sys/class/infiniband"
)

// errCounterNotAvailable is returned when a hw_counter file does not exist
// or is not supported by the current hardware/driver revision.
var errCounterNotAvailable = errors.New("counter not available")

// efaDevice represents a single EFA device with its port and counter values.
type efaDevice struct {
	name     string
	port     string
	eniID    string // resolved ENI ID, empty if resolution failed
	counters map[string]uint64
}

// sysFsReader is an interface for reading EFA data from sysfs, enabling testability.
type sysFsReader interface {
	EfaDataExists() (bool, error)
	ListDevices() ([]string, error)
	ListPorts(deviceName string) ([]string, error)
	ReadCounter(deviceName, port, counter string) (uint64, error)
	ReadGID(deviceName string) (string, error)
}

type sysfsReaderImpl struct {
	basePath string
	logger   *zap.Logger
}

func newSysFsReader(hostPath string, logger *zap.Logger) sysFsReader {
	basePath := defaultEfaPath
	if hostPath != "" {
		basePath = filepath.Join(hostPath, defaultEfaPath)
	}
	return &sysfsReaderImpl{basePath: basePath, logger: logger}
}

func (r *sysfsReaderImpl) EfaDataExists() (bool, error) {
	info, err := os.Stat(r.basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if !info.IsDir() {
		return false, nil
	}

	if err := checkPermissions(info); err != nil {
		r.logger.Warn("Not reading from EFA directory, permission check failed",
			zap.String("path", r.basePath), zap.Error(err))
		return false, nil
	}

	return true, nil
}

func (r *sysfsReaderImpl) ListDevices() ([]string, error) {
	dirs, err := os.ReadDir(r.basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to list EFA devices at %q: %w", r.basePath, err)
	}

	result := make([]string, 0, len(dirs))
	for _, entry := range dirs {
		// In sysfs, entries are often symlinks to device directories.
		// Follow symlinks with os.Stat to check the real target.
		info, err := os.Stat(filepath.Join(r.basePath, entry.Name()))
		if err != nil {
			continue
		}
		if !info.IsDir() {
			continue
		}
		result = append(result, entry.Name())
	}
	return result, nil
}

func (r *sysfsReaderImpl) ListPorts(deviceName string) ([]string, error) {
	portsPath := filepath.Join(r.basePath, deviceName, "ports")
	portDirs, err := os.ReadDir(portsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to list EFA ports at %q: %w", portsPath, err)
	}

	result := make([]string, 0, len(portDirs))
	for _, dir := range portDirs {
		info, err := os.Stat(filepath.Join(portsPath, dir.Name()))
		if err != nil {
			continue
		}
		if !info.IsDir() {
			continue
		}
		result = append(result, dir.Name())
	}
	return result, nil
}

// ReadGID reads the first GID (index 0) from port 1 of the given device.
// Port 1 is hardcoded because EFA is a single-port device, and the GID
// represents the device-level IPv6 link-local address (derived from the
// underlying NIC's MAC), which is the same across all ports.
func (r *sysfsReaderImpl) ReadGID(deviceName string) (string, error) {
	gidPath := filepath.Join(r.basePath, deviceName, "ports", "1", "gids", "0")
	data, err := os.ReadFile(gidPath)
	if err != nil {
		return "", fmt.Errorf("failed to read GID file %q: %w", gidPath, err)
	}
	return strings.TrimSpace(string(data)), nil
}

func (r *sysfsReaderImpl) ReadCounter(deviceName, port, counter string) (uint64, error) {
	path := filepath.Join(r.basePath, deviceName, "ports", port, "hw_counters", counter)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, errCounterNotAvailable
		}
		if os.IsPermission(err) {
			r.logger.Warn("Permission denied reading EFA counter",
				zap.String("path", path), zap.Error(err))
			return 0, errCounterNotAvailable
		}
		if errors.Is(err, syscall.EOPNOTSUPP) || errors.Is(err, syscall.EINVAL) {
			return 0, errCounterNotAvailable
		}
		return 0, fmt.Errorf("failed to read file %q: %w", path, err)
	}

	value := strings.TrimSpace(string(data))
	if strings.Contains(value, "N/A (no PMA)") {
		return 0, errCounterNotAvailable
	}

	v, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse %q from %q: %w", value, path, err)
	}
	return v, nil
}
