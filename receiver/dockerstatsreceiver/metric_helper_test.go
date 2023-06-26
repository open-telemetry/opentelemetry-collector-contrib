// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dockerstatsreceiver

import (
	"errors"
	"testing"

	ctypes "github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
)

func Test_calculateCPULimit1(t *testing.T) {
	tests := []struct {
		name string
		args *ctypes.HostConfig
		want float64
		err  error
	}{
		{
			"Test CPULimit",
			&ctypes.HostConfig{
				Resources: ctypes.Resources{
					NanoCPUs: 2500000000,
				},
			},
			2.5,
			nil,
		},
		{
			"Test CPUSetCpu",
			&ctypes.HostConfig{
				Resources: ctypes.Resources{
					CpusetCpus: "0-2",
				},
			},
			3,
			nil,
		},
		{
			"Test CPUQuota",
			&ctypes.HostConfig{
				Resources: ctypes.Resources{
					CPUQuota: 50000,
				},
			},
			0.5,
			nil,
		},
		{
			"Test CPUQuota Custom Period",
			&ctypes.HostConfig{
				Resources: ctypes.Resources{
					CPUQuota:  300000,
					CPUPeriod: 200000,
				},
			},
			1.5,
			nil,
		},
		{
			"Test CPUSetCpu Error",
			&ctypes.HostConfig{
				Resources: ctypes.Resources{
					CpusetCpus: "0-a",
				},
			},
			0,
			errors.New("invalid cpusetCpus value"),
		},
		{
			"Test Default",
			&ctypes.HostConfig{
				Resources: ctypes.Resources{
					NanoCPUs:   1800000000,
					CpusetCpus: "0-1",
					CPUQuota:   400000,
				},
			},
			1.8,
			nil,
		},
		{
			"Test No Values",
			&ctypes.HostConfig{
				Resources: ctypes.Resources{},
			},
			0,
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want, err := calculateCPULimit(tt.args)
			assert.Equalf(t, tt.want, want, "calculateCPULimit(%v)", tt.args)
			assert.Equalf(t, tt.err, err, "calculateCPULimit(%v)", tt.args)
		})
	}
}

func Test_parseCPUSet(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
		err      error
	}{
		{"0,2", 2, nil},
		{"0-2", 3, nil},
		{"0-2,4", 4, nil},
		{"0-2,4-5", 5, nil},
		{"a-b", 0, errors.New("invalid cpusetCpus value")},
		{"", 1, nil},
	}

	for _, test := range tests {
		result, err := parseCPUSet(test.input)
		assert.Equal(t, test.expected, result)
		assert.Equal(t, test.err, err)
	}
}
